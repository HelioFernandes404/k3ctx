"""CLI presenters for human-readable output."""

from __future__ import annotations

import sys
from collections.abc import Mapping, Sequence
from typing import Any

from src.domain.discovery import ClientPage, HostPage, HostResolutionResult
from src.domain.models import ConnectResult
from src.tunnel import build_sshuttle_command

SECTION_SEPARATOR = "=" * 60
GREEN = "\033[0;32m"
RED = "\033[0;31m"
YELLOW = "\033[1;33m"
NC = "\033[0m"


def _print_section(title: str) -> None:
    print(f"\n{SECTION_SEPARATOR}")
    print(title)
    print(SECTION_SEPARATOR)


def print_single_success(result: ConnectResult) -> None:
    print(f"\n✓ Context '{result.context_name}' configured")
    print(f"✓ Kubeconfig available on localhost:{result.local_port}")
    if result.used_cache:
        print("✓ Using cached kubeconfig")
    else:
        print("✓ Fetched kubeconfig from remote")
    if result.tunnel_pid is not None:
        print(f"✓ Tunnel ready (PID: {result.tunnel_pid})")
    if result.argocd_local_port is not None:
        print(f"✓ ArgoCD available at localhost:{result.argocd_local_port}")

    requirement = result.network_requirement
    if requirement.needs_vpn:
        print("\n⚠️  Remember: This context requires VPN to access the cluster.")
    if requirement.type == "sshuttle":
        print("\n🔒 Remember: Keep sshuttle running to access this cluster.")
        if requirement.network_range:
            print(f"   {build_sshuttle_command(requirement.type, requirement.network_range)}")

    print("\nYou can now use kubectl/k9s directly!")
    print("  kubectl get nodes")
    print("  k9s -l debug")
    print(f"\nTo switch contexts later:\n  kubectl config use-context {result.context_name}")


def print_single_failure(result: ConnectResult) -> None:
    if result.error is None:
        print("Failed to connect.", file=sys.stderr)
        return
    print(f"Failed to connect: {result.error.message}")


def _network_note(result: ConnectResult) -> str:
    requirement = result.network_requirement
    if requirement.type == "sshuttle" and requirement.needs_vpn:
        return " ⚠ requires VPN + sshuttle"
    if requirement.type == "sshuttle":
        return " ⚠ requires sshuttle"
    if requirement.needs_vpn:
        return " ⚠ requires VPN"
    return ""


def print_multi_summary(results: Sequence[ConnectResult]) -> list[ConnectResult]:
    successful = [result for result in results if result.success]
    failed = [result for result in results if not result.success]

    _print_section("Connection Summary")
    print(f"\n✓ Connected: {len(successful)}/{len(results)} clusters")

    for result in successful:
        print(
            f"  ✓ {result.context_name} (localhost:{result.local_port})"
            f"{_network_note(result)}"
        )

    if failed:
        print(f"\n✗ Failed: {len(failed)} clusters")
        for result in failed:
            message = "Connection failed"
            if result.error is not None:
                message = result.error.message
            print(f"  ✗ {result.context_name} - {message}")

    return successful


def print_network_reminders(results: Sequence[ConnectResult]) -> None:
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


def print_multi_usage() -> None:
    _print_section("Usage:")
    print("  kubectl config use-context <name>  # Switch between clusters")
    print("  make k9s                           # Launch k9s")
    print("  make status                        # Show connection status")
    print("  make tunnel-list                   # List active tunnels")


def print_client_page(page: ClientPage) -> None:
    if not page.items:
        print("No clients found.")
        return

    print("CLIENT".ljust(28) + "HOSTS")
    for item in page.items:
        print(f"{item.client:<28}{item.host_count}")

    print(f"\nShowing {page.page.returned} of {page.page.total}")
    if page.page.has_more and page.page.next_cursor is not None:
        print(f"Next page: --cursor {page.page.next_cursor}")


def print_host_page(page: HostPage) -> None:
    if not page.items:
        print("No hosts found.")
        return

    print("HOST NAME".ljust(24) + "SYSTEMFRAME ID".ljust(20) + "IP".ljust(18) + "GROUP")
    for item in page.items:
        print(
            f"{item.host_name:<24}"
            f"{(item.systemframe_id or '-'): <20}"
            f"{(item.addr_ip or '-'): <18}"
            f"{item.group}"
        )

    print(f"\nShowing {page.page.returned} of {page.page.total}")
    if page.page.has_more and page.page.next_cursor is not None:
        print(f"Next page: --cursor {page.page.next_cursor}")


def print_host_resolution_failure(
    result: HostResolutionResult,
    *,
    cli_name: str,
) -> None:
    print(result.hint or "Host resolution failed.")

    if not result.matches:
        return

    print()
    print("HOST NAME".ljust(24) + "SYSTEMFRAME ID".ljust(20) + "IP".ljust(18) + "CONTEXT")
    for item in result.matches:
        print(
            f"{item.host_name:<24}"
            f"{(item.systemframe_id or '-'): <20}"
            f"{(item.addr_ip or '-'): <18}"
            f"{item.context_name}"
        )

    if (
        result.page.has_more
        and result.page.next_cursor is not None
        and result.query.client is not None
    ):
        print(
            f"\nNext page: {cli_name} hosts "
            f"{result.query.client} --cursor {result.page.next_cursor}"
        )


def _format_network_warning(
    network_meta: Mapping[str, Any],
    network_validation: Mapping[str, Any],
) -> str:
    warning_message = str(network_validation.get("warning") or "").lower()

    if "could not be read safely" in warning_message:
        return f" {YELLOW}⚠ network metadata unreadable{NC}"
    if not network_validation.get("ok", True):
        if network_meta.get("needs_vpn") or "vpn" in warning_message:
            return f" {YELLOW}⚠ requires VPN{NC}"
        if network_meta.get("network_type") == "sshuttle" or "sshuttle" in warning_message:
            return f" {YELLOW}⚠ requires sshuttle{NC}"

    return ""


def print_status(
    items: Sequence[Mapping[str, object]],
    validations: Mapping[str, Mapping[str, Any]],
) -> None:
    if not items:
        print(f"{YELLOW}No connected clusters found.{NC}")
        print("\nRun: make run  # Connect a cluster")
        return

    print(f"{GREEN}Connected clusters:{NC}")
    current_context: str | None = None
    network_requirements: list[Mapping[str, Any]] = []

    for item in items:
        name = str(item["name"])
        if item.get("is_current"):
            current_context = name
        network_validation = validations.get(name, {})
        network_meta = item.get("network_metadata")
        network_meta_mapping = network_meta if isinstance(network_meta, Mapping) else {}
        network_warning = _format_network_warning(network_meta_mapping, network_validation)

        if (
            not network_validation.get("ok", True)
            and network_meta_mapping.get("network_type") == "sshuttle"
        ):
            network_requirements.append(network_meta_mapping)

        current_marker = " (active)" if item.get("is_current") else ""
        if item.get("tunnel_running"):
            print(
                f"  {GREEN}✓{NC} {name} (localhost:{item.get('local_port')}) "
                f"[PID: {item.get('tunnel_pid')}]{network_warning}{current_marker}"
            )
        else:
            print(f"  {RED}✗{NC} {name} (tunnel down){network_warning}{current_marker}")

    if current_context:
        print(f"\n{GREEN}Current context:{NC} {current_context}")
    else:
        print(f"\n{YELLOW}No current context set{NC}")

    if network_requirements:
        print(f"\n{YELLOW}Active network requirements:{NC}")
        shown_commands: set[str] = set()
        for meta in network_requirements:
            cmd = meta.get("sshuttle_command")
            if isinstance(cmd, str) and cmd and cmd not in shown_commands:
                print(f"  🔒 {cmd}")
                shown_commands.add(cmd)

    print(f"\n{GREEN}Commands:{NC}")
    print("  kubectl config use-context <name>  # Switch context")
    print("  make k9s                           # Launch k9s")
    print("  make tunnel-list                   # List tunnels")


__all__ = [
    "print_client_page",
    "print_host_page",
    "print_host_resolution_failure",
    "print_multi_summary",
    "print_multi_usage",
    "print_network_reminders",
    "print_single_failure",
    "print_single_success",
    "print_status",
]
