#!/usr/bin/env python3
"""Manual single-cluster connection flow backed by shared services."""

from __future__ import annotations

import os
import sys
from pathlib import Path
from typing import Any

from dotenv import load_dotenv

from src.cli import (
    NonInteractiveTerminalError,
    confirm_action,
    select_company,
    select_host,
)
from src.config import load_effective_config
from src.inventory import update_inventory_repo
from src.logging_config import setup_logging
from src.models import ClusterTarget, ConnectResult
from src.network import check_network_requirement, check_vpn_requirement, is_private_network
from src.services.connect import connect_cluster
from src.tunnel import get_unique_port

load_dotenv()

PROJECT_DIR = Path(__file__).parent


def _confirm_action_or_none(prompt: str, *, default: bool) -> bool | None:
    try:
        return confirm_action(prompt, default=default)
    except KeyboardInterrupt:
        return None


def build_target(company: str, host_alias: str, host_info: dict[str, Any]) -> ClusterTarget:
    return ClusterTarget(
        company=company,
        host_alias=host_alias,
        group=host_info["group"],
        host_config=host_info.get("config", {}),
        group_vars=host_info.get("group_vars", {}),
    )


def _show_manual_network_warning(
    host_alias: str,
    host_info: dict[str, Any],
    inv_data: dict[str, Any],
) -> bool:
    group = host_info["group"]
    needs_vpn = check_vpn_requirement(inv_data, group, host_alias)
    network_type, network_range = check_network_requirement(host_alias, host_info)

    if needs_vpn:
        print("\n⚠️  WARNING: This host requires VPN (argocd_use_socks5_proxy=true)")
        print("   Make sure your VPN is connected before proceeding.")
        confirmed = _confirm_action_or_none("Continue?", default=False)
        if not confirmed:
            return False

    if network_type == "sshuttle":
        print(f"\n🔒 NETWORK REQUIREMENT: This host is on private network {network_range}")
        print("   You need to run sshuttle to access this network.")
        print("\n   Example command:")
        print(f"   sshuttle -v -r helio@100.64.5.10 {network_range}")
        print("\n   Make sure sshuttle is running before proceeding.")
        confirmed = _confirm_action_or_none("Continue?", default=False)
        if not confirmed:
            return False

    return True


def _print_success(result: ConnectResult) -> None:
    print(f"\n✓ Context '{result.context_name}' configured")
    print(f"✓ Kubeconfig available on localhost:{result.local_port}")
    if result.used_cache:
        print("✓ Using cached kubeconfig")
    else:
        print("✓ Fetched kubeconfig from remote")
    if result.tunnel_pid is not None:
        print(f"✓ Tunnel ready (PID: {result.tunnel_pid})")

    requirement = result.network_requirement
    if requirement.needs_vpn:
        print("\n⚠️  Remember: This context requires VPN to access the cluster.")
    if requirement.type == "sshuttle":
        print("\n🔒 Remember: Keep sshuttle running to access this cluster.")
        if requirement.network_range:
            print(f"   sshuttle -v -r helio@100.64.5.10 {requirement.network_range}")

    print("\nYou can now use kubectl/k9s directly!")
    print("  kubectl get nodes")
    print("  k9s -l debug")
    print(f"\nTo switch contexts later:\n  kubectl config use-context {result.context_name}")


def _print_failure(result: ConnectResult) -> None:
    if result.error is None:
        print("Failed to connect.", file=sys.stderr)
        return

    print(f"Failed to connect: {result.error.message}")


def main() -> int:
    log_file_path = os.path.expanduser(
        os.getenv("K9S_LOG_FILE", "~/.local/state/k9s/k9s-config.log")
    )
    setup_logging(log_file=log_file_path, structured=True)

    try:
        config = load_effective_config(PROJECT_DIR, os.getenv("CONFIG_FILE"))

        if config.inventory_path.exists():
            print("Updating inventory repository...")
            success, message = update_inventory_repo(config.inventory_path)
            if success:
                print(f"✓ {message}")
            else:
                print(f"⚠️  {message} (continuing with local version)")

        while True:
            try:
                company, inv_data = select_company(config.inventory_path)
            except SystemExit as exc:
                return int(exc.code or 1)

            if company is None:
                print("Cancelled.")
                return 0
            assert inv_data is not None

            while True:
                try:
                    host_alias, host_info = select_host(company, inv_data)
                except SystemExit as exc:
                    return int(exc.code or 1)

                if host_alias is None:
                    break
                assert host_info is not None

                if not _show_manual_network_warning(host_alias, host_info, inv_data):
                    continue

                result = connect_cluster(
                    target=build_target(company, host_alias, host_info),
                    config=config,
                    allow_manual_network=True,
                )

                if result.success:
                    _print_success(result)
                    return 0

                _print_failure(result)
                retry = _confirm_action_or_none("Try another host?", default=True)
                if retry:
                    continue
                if retry is None:
                    return 0
                return 1

    except NonInteractiveTerminalError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
