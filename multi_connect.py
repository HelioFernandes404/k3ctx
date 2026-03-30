#!/usr/bin/env python3
"""Manual multi-cluster connection flow backed by shared services."""

from __future__ import annotations

import os
import sys
from pathlib import Path

from dotenv import load_dotenv
import questionary

from src.cli import NonInteractiveTerminalError, custom_style, require_interactive_terminal
from src.config import load_effective_config
from src.logging_config import setup_logging
from src.models import ClusterTarget, ConnectResult, NetworkRequirement
from src.network import detect_network_requirement
from src.services.connect import connect_multiple
from src.services.contexts import set_current_context
from src.services.inventory_service import list_cluster_targets
from src.tunnel import build_sshuttle_command

load_dotenv()

PROJECT_DIR = Path(__file__).parent
SECTION_SEPARATOR = "=" * 60


def get_network_requirement(target: ClusterTarget) -> NetworkRequirement:
    network_type, network_range, needs_vpn = detect_network_requirement(
        target.host_config,
        target.group_vars,
        target.group,
    )
    return NetworkRequirement(
        type=network_type,
        network_range=network_range,
        needs_vpn=needs_vpn,
    )


def format_cluster_label(target: ClusterTarget) -> str:
    requirement = get_network_requirement(target)
    indicators: list[str] = []
    if requirement.needs_vpn:
        indicators.append("[VPN]")
    if requirement.type == "sshuttle":
        indicators.append("[sshuttle]")

    label = f"{target.company}: {target.host_alias}"
    if indicators:
        label = f"{label} {' '.join(indicators)}"
    return label


def _print_section(title: str) -> None:
    print(f"\n{SECTION_SEPARATOR}")
    print(title)
    print(SECTION_SEPARATOR)


def _network_note(requirement: NetworkRequirement) -> str:
    if requirement.type == "sshuttle" and requirement.needs_vpn:
        return " ⚠ requires VPN + sshuttle"
    if requirement.type == "sshuttle":
        return " ⚠ requires sshuttle"
    if requirement.needs_vpn:
        return " ⚠ requires VPN"
    return ""


def show_network_warnings(selected_targets: list[ClusterTarget]) -> bool:
    direct: list[ClusterTarget] = []
    vpn_required: list[ClusterTarget] = []
    sshuttle_required: list[tuple[ClusterTarget, NetworkRequirement]] = []

    for target in selected_targets:
        requirement = get_network_requirement(target)
        has_manual_requirement = False
        if requirement.needs_vpn:
            vpn_required.append(target)
            has_manual_requirement = True
        if requirement.type == "sshuttle":
            sshuttle_required.append((target, requirement))
            has_manual_requirement = True
        if not has_manual_requirement:
            direct.append(target)

    _print_section("Selected clusters:")

    if direct:
        print("\n✓ Direct access (no special setup):")
        for target in direct:
            print(f"  • {target.company}: {target.host_alias}")

    if sshuttle_required:
        print("\n⚠ Requires sshuttle:")
        for target, requirement in sshuttle_required:
            print(f"  • {target.company}: {target.host_alias} → {requirement.network_range}")

    if vpn_required:
        print("\n⚠ Requires VPN:")
        for target in vpn_required:
            print(f"  • {target.company}: {target.host_alias}")

    if sshuttle_required or vpn_required:
        _print_section("Network setup commands:")

        if sshuttle_required:
            commands = {
                build_sshuttle_command(requirement.type, requirement.network_range)
                for _, requirement in sshuttle_required
            }
            for command in sorted(cmd for cmd in commands if cmd):
                print(f"\n🔒 {command}")

        if vpn_required:
            print("\n🔐 Ensure VPN connection is active before proceeding")

    print(f"\n{SECTION_SEPARATOR}")

    try:
        require_interactive_terminal()
        confirmed = questionary.confirm(
            "Continue with multi-cluster connection?",
            default=True,
            style=custom_style,
        ).ask()
    except KeyboardInterrupt:
        return False

    return bool(confirmed)


def select_clusters_interactive(targets: list[ClusterTarget]) -> list[ClusterTarget]:
    require_interactive_terminal()

    selected: list[ClusterTarget] = []
    label_to_target = {format_cluster_label(target): target for target in targets}
    labels = list(label_to_target.keys())
    selected_labels: set[str] = set()

    print(f"\n📋 Available: {len(labels)} clusters")
    print("💡 Tip: Type to search, Enter to add, Ctrl+C quando terminar\n")

    while True:
        remaining = [
            label
            for label in labels
            if label not in selected_labels
        ]
        if not remaining:
            print("All clusters selected!")
            break

        if selected:
            print(f"\n✓ Selected ({len(selected)}):")
            for index, target in enumerate(selected, start=1):
                print(f"  {index}. {format_cluster_label(target)}")
            print()

        try:
            choice = questionary.autocomplete(
                f"Add cluster (type to search, {len(remaining)} remaining):",
                choices=remaining,
                match_middle=True,
                style=custom_style,
            ).ask()
        except KeyboardInterrupt:
            print()
            break

        if choice is None:
            break

        if choice in label_to_target:
            target = label_to_target[choice]
            selected.append(target)
            selected_labels.add(choice)
            print(f"  ✓ Added: {choice}")

    return selected


def _print_summary(results: list[ConnectResult]) -> list[ConnectResult]:
    successful = [result for result in results if result.success]
    failed = [result for result in results if not result.success]

    _print_section("Connection Summary")
    print(f"\n✓ Connected: {len(successful)}/{len(results)} clusters")

    for result in successful:
        print(
            f"  ✓ {result.context_name} (localhost:{result.local_port})"
            f"{_network_note(result.network_requirement)}"
        )

    if failed:
        print(f"\n✗ Failed: {len(failed)} clusters")
        for result in failed:
            message = "Connection failed"
            if result.error is not None:
                message = result.error.message
            print(f"  ✗ {result.context_name} - {message}")

    return successful


def _print_network_reminders(results: list[ConnectResult]) -> None:
    vpn_required = any(result.network_requirement.needs_vpn for result in results)
    commands = {
        build_sshuttle_command(
            result.network_requirement.type,
            result.network_requirement.network_range,
        )
        for result in results
        if result.network_requirement.type == "sshuttle"
    }
    active_commands = sorted(command for command in commands if command)
    if not active_commands and not vpn_required:
        return

    _print_section("⚠ Active network requirements:")
    for command in active_commands:
        print(f"  🔒 {command}")
    if vpn_required:
        print("  🔐 Ensure VPN connection is active before proceeding")


def main() -> int:
    log_file_path = os.path.expanduser(
        os.getenv("K9S_LOG_FILE", "~/.local/state/k9s/k9s-config.log")
    )
    setup_logging(log_file=log_file_path, structured=True)

    try:
        config = load_effective_config(PROJECT_DIR, os.getenv("CONFIG_FILE"))
        print("Loading available clusters...")
        targets = list_cluster_targets(config.inventory_path)
        if not targets:
            print("No clusters found in inventory.", file=sys.stderr)
            return 1

        print(f"Found {len(targets)} clusters in inventory")

        selected = select_clusters_interactive(targets)
        if not selected:
            print("\nNo clusters selected. Cancelled.")
            return 0

        if not show_network_warnings(selected):
            print("Cancelled.")
            return 0

        _print_section("Connecting to clusters...")
        results = connect_multiple(
            targets=selected,
            config=config,
            allow_manual_network=True,
        )
        successful = _print_summary(results)
        if not successful:
            print("\nNo clusters connected successfully.")
            return 1

        first_context = successful[0].context_name
        print(f"\nSetting active context to: {first_context}")
        context_error = set_current_context(
            first_context,
            require_confirmation=False,
            confirmed=True,
        )
        if context_error is None:
            print(f"✓ Active context: {first_context}")
        else:
            print("⚠ Failed to set active context (you can set it manually)")

        _print_network_reminders(successful)

        _print_section("Usage:")
        print("  kubectl config use-context <name>  # Switch between clusters")
        print("  make k9s                           # Launch k9s")
        print("  make status                        # Show connection status")
        print("  make tunnel-list                   # List active tunnels")
        return 0

    except NonInteractiveTerminalError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
