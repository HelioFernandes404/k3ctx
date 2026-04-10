"""Official CLI entrypoint built on the shared application layer."""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

from dotenv import load_dotenv

from src.application.use_cases.connect import connect_cluster, connect_multiple
from src.application.use_cases.contexts import set_current_context
from src.application.use_cases.inventory import list_cluster_targets, refresh_inventory_if_possible
from src.application.use_cases.status import list_context_status, validate_context_network
from src.bootstrap import ServiceContainer, build_service_container
from src.config import load_effective_config
from src.domain.models import EffectiveConfig
from src.interfaces.cli.presenters import (
    print_multi_summary,
    print_multi_usage,
    print_network_reminders,
    print_single_failure,
    print_single_success,
    print_status,
    show_manual_network_warnings,
    show_network_warnings,
)
from src.interfaces.cli.prompts import (
    NonInteractiveTerminalError,
    confirm_action,
    select_company,
    select_multiple_targets,
    select_single_target,
)
from src.logging_config import setup_logging

PROJECT_DIR = Path(__file__).resolve().parents[3]


def _configure_logging() -> None:
    log_file_path = os.path.expanduser(
        os.getenv("K9S_LOG_FILE", "~/.local/state/k9s/k9s-config.log")
    )
    setup_logging(log_file=log_file_path, structured=True)


def _load_runtime() -> tuple[EffectiveConfig, ServiceContainer]:
    config = load_effective_config(PROJECT_DIR, os.getenv("CONFIG_FILE"))
    services = build_service_container()
    return config, services


def _refresh_inventory(config: EffectiveConfig, services: ServiceContainer) -> None:
    refresh_result = refresh_inventory_if_possible(config.inventory_path, services.refresher)
    if refresh_result is None or not isinstance(refresh_result, tuple) or len(refresh_result) != 2:
        return

    print("Updating inventory repository...")
    success, message = refresh_result
    if success:
        print(f"✓ {message}")
    else:
        print(f"⚠️  {message} (continuing with local version)")


def _confirm_action_or_none(prompt: str, *, default: bool) -> bool | None:
    try:
        return confirm_action(prompt, default=default)
    except KeyboardInterrupt:
        return None


def run_single() -> int:
    config, services = _load_runtime()
    _refresh_inventory(config, services)
    targets = list_cluster_targets(config.inventory_path, services.catalog)
    if not targets:
        print(f"No inventories found in {config.inventory_path}.", file=sys.stderr)
        return 1

    companies = sorted({target.company for target in targets})
    while True:
        company = select_company(companies)
        if company is None:
            print("Cancelled.")
            return 0

        company_targets = [target for target in targets if target.company == company]
        while True:
            target = select_single_target(company, company_targets)
            if target is None:
                break

            if not show_manual_network_warnings(target):
                continue

            result = connect_cluster(
                target=target,
                config=config,
                connector=services.connector,
                allow_manual_network=True,
            )
            if result.success:
                print_single_success(result)
                return 0

            print_single_failure(result)
            retry = _confirm_action_or_none("Try another host?", default=True)
            if retry:
                continue
            if retry is None:
                return 0
            return 1


def run_multi() -> int:
    config, services = _load_runtime()
    _refresh_inventory(config, services)
    print("Loading available clusters...")
    targets = list_cluster_targets(config.inventory_path, services.catalog)
    if not targets:
        print("No clusters found in inventory.", file=sys.stderr)
        return 1

    print(f"Found {len(targets)} clusters in inventory")
    selected = select_multiple_targets(targets)
    if not selected:
        print("\nNo clusters selected. Cancelled.")
        return 0

    if not show_network_warnings(selected):
        print("Cancelled.")
        return 0

    print("\n============================================================")
    print("Connecting to clusters...")
    print("============================================================")
    results = connect_multiple(
        targets=selected,
        config=config,
        connector=services.connector,
        allow_manual_network=True,
    )
    successful = print_multi_summary(results)
    if not successful:
        print("\nNo clusters connected successfully.")
        return 1

    first_context = successful[0].context_name
    print(f"\nSetting active context to: {first_context}")
    context_error = set_current_context(
        first_context,
        switcher=services.switcher,
        require_confirmation=False,
        confirmed=True,
    )
    if context_error is None:
        print(f"✓ Active context: {first_context}")
    else:
        print("⚠️  Failed to set active context (you can set it manually)")

    print_network_reminders(successful)
    print_multi_usage()
    return 0


def run_status() -> int:
    _, services = _load_runtime()
    items = list_context_status(services.status_reader)
    validations = {
        str(item["name"]): validate_context_network(
            str(item["name"]),
            services.status_reader,
        )
        for item in items
    }
    print_status(items, validations)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="k3s-context-tunnel-manager")
    subparsers = parser.add_subparsers(dest="command")
    subparsers.add_parser("single")
    subparsers.add_parser("multi")
    subparsers.add_parser("status")
    return parser


def main(argv: list[str] | None = None) -> int:
    load_dotenv()
    parser = build_parser()
    args = parser.parse_args(argv)
    command = args.command or "single"

    _configure_logging()

    try:
        if command == "single":
            return run_single()
        if command == "multi":
            return run_multi()
        if command == "status":
            return run_status()
    except NonInteractiveTerminalError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1

    parser.print_help()
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
