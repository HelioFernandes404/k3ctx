"""Official CLI entrypoint built on the shared application layer."""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path

from dotenv import load_dotenv

from src.application.use_cases.connect import connect_cluster, connect_multiple
from src.application.use_cases.contexts import set_current_context
from src.application.use_cases.discovery import (
    list_client_summaries,
    load_host_records,
    resolve_host,
    search_hosts,
)
from src.application.use_cases.inventory import (
    find_target_by_context_name,
    list_cluster_targets,
    refresh_inventory_if_possible,
)
from src.application.use_cases.status import list_context_status, validate_context_network
from src.bootstrap import ServiceContainer, build_service_container
from src.config import load_effective_config
from src.domain.discovery import HostQuery, HostRecord
from src.domain.models import EffectiveConfig
from src.interfaces.cli.presenters import (
    print_client_page,
    print_host_page,
    print_host_resolution_failure,
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
from src.interfaces.serialization import (
    client_page_payload,
    host_page_payload,
    host_resolution_payload,
    to_jsonable,
)
from src.logging_config import setup_logging

PROJECT_DIR = Path(__file__).resolve().parents[3]
_IP_PATTERN = re.compile(r"^\d{1,3}(?:\.\d{1,3}){3}$")


def _cli_name() -> str:
    return Path(sys.argv[0]).name or "context-tunnel-manager"


def _configure_logging() -> None:
    log_file_path = os.path.expanduser(
        os.getenv("K9S_LOG_FILE", "~/.local/state/k9s/k9s-config.log")
    )
    setup_logging(log_file=log_file_path, structured=True)


def _load_runtime() -> tuple[EffectiveConfig, ServiceContainer]:
    config = load_effective_config(PROJECT_DIR, os.getenv("CONFIG_FILE"))
    services = build_service_container()
    return config, services


def _has_tty() -> bool:
    stdin_isatty = getattr(sys.stdin, "isatty", lambda: False)()
    stdout_isatty = getattr(sys.stdout, "isatty", lambda: False)()
    return bool(stdin_isatty and stdout_isatty)


def _refresh_inventory(
    config: EffectiveConfig,
    services: ServiceContainer,
    *,
    quiet: bool = False,
) -> None:
    refresh_result = refresh_inventory_if_possible(config.inventory_path, services.refresher)
    if refresh_result is None or not isinstance(refresh_result, tuple) or len(refresh_result) != 2:
        return

    if quiet:
        return

    success, message = refresh_result
    if success:
        print(f"✓ {message}")
    else:
        print(f"⚠️  {message} (continuing with local version)")


def _print_json(payload: object) -> None:
    print(json.dumps(to_jsonable(payload), sort_keys=True))


def _confirm_action_or_none(prompt: str, *, default: bool) -> bool | None:
    try:
        return confirm_action(prompt, default=default)
    except KeyboardInterrupt:
        return None


def _guided_connect() -> int:
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
        target = select_single_target(company, company_targets)
        if target is None:
            continue
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


def _build_connect_query(
    *,
    identifiers: list[str],
    client: str | None,
    host_name: str | None,
    systemframe_id: str | None,
    addr_ip: str | None,
    context_name: str | None,
    known_records: list[HostRecord],
) -> HostQuery:
    if len(identifiers) > 3:
        raise ValueError("connect accepts at most three identifiers")

    known_clients = {record.client for record in known_records}
    known_contexts = {record.context_name for record in known_records}

    resolved_client = client
    resolved_host = host_name
    resolved_id = systemframe_id
    resolved_ip = addr_ip
    resolved_context = context_name

    for token in identifiers:
        if _IP_PATTERN.match(token) and resolved_ip is None:
            resolved_ip = token
        elif token in known_contexts and resolved_context is None:
            resolved_context = token
        elif token in known_clients and resolved_client is None:
            resolved_client = token
        elif resolved_host is None:
            resolved_host = token
        elif resolved_id is None:
            resolved_id = token
        else:
            raise ValueError("Unable to infer identifier roles. Use explicit flags.")

    return HostQuery(
        client=resolved_client,
        host_name=resolved_host,
        systemframe_id=resolved_id,
        addr_ip=resolved_ip,
        context_name=resolved_context,
    )


def run_clients(args: argparse.Namespace) -> int:
    config, services = _load_runtime()
    _refresh_inventory(config, services, quiet=args.json)
    page = list_client_summaries(
        config.inventory_path,
        services.catalog,
        query=args.query,
        limit=args.limit,
        cursor=args.cursor,
    )
    if args.json:
        _print_json(client_page_payload(page))
    else:
        print_client_page(page)
    return 0


def run_hosts(args: argparse.Namespace) -> int:
    config, services = _load_runtime()
    _refresh_inventory(config, services, quiet=args.json)
    page = search_hosts(
        config.inventory_path,
        services.catalog,
        query=HostQuery(
            client=args.client,
            host_name=args.host,
            systemframe_id=args.systemframe_id,
            addr_ip=args.addr_ip,
            query=args.query,
        ),
        limit=args.limit,
        cursor=args.cursor,
    )
    if args.json:
        _print_json(host_page_payload(page))
    else:
        print_host_page(page)
    return 0


def run_connect(args: argparse.Namespace) -> int:
    if not args.identifiers and not any(
        [args.client, args.host, args.systemframe_id, args.addr_ip, args.context_name]
    ):
        if not _has_tty():
            print(
                "Error: connect requires at least one identifier in non-interactive mode.",
                file=sys.stderr,
            )
            return 4
        return _guided_connect()

    config, services = _load_runtime()
    _refresh_inventory(config, services, quiet=args.json)
    records = load_host_records(config.inventory_path, services.catalog)
    query = _build_connect_query(
        identifiers=args.identifiers,
        client=args.client,
        host_name=args.host,
        systemframe_id=args.systemframe_id,
        addr_ip=args.addr_ip,
        context_name=args.context_name,
        known_records=records,
    )
    resolution = resolve_host(
        config.inventory_path,
        services.catalog,
        query=query,
        limit=10,
    )

    if resolution.status != "unique" or resolution.context_name is None:
        if args.json:
            _print_json(host_resolution_payload(resolution, cli_name=_cli_name()))
        else:
            print_host_resolution_failure(resolution, cli_name=_cli_name())
        return 3 if resolution.status == "ambiguous" else 2

    target = find_target_by_context_name(
        resolution.context_name,
        config.inventory_path,
        services.catalog,
    )
    if target is None:
        print("Resolved context disappeared from inventory.", file=sys.stderr)
        return 1

    allow_manual_network = _has_tty() and not args.json
    if allow_manual_network and not show_manual_network_warnings(target):
        return 1

    result = connect_cluster(
        target=target,
        config=config,
        connector=services.connector,
        allow_manual_network=allow_manual_network,
    )
    if args.json:
        _print_json(result.to_public_dict())
    else:
        if result.success:
            print_single_success(result)
        else:
            print_single_failure(result)
    return 0 if result.success else 1


def run_status(args: argparse.Namespace) -> int:
    _, services = _load_runtime()
    items = list_context_status(services.status_reader)
    if args.json:
        _print_json(items)
        return 0

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
    parser = argparse.ArgumentParser(prog=_cli_name())
    subparsers = parser.add_subparsers(dest="command")

    clients_parser = subparsers.add_parser("clients", help="List clients with host counts")
    clients_parser.add_argument("query", nargs="?")
    clients_parser.add_argument("--limit", type=int, default=20)
    clients_parser.add_argument("--cursor")
    clients_parser.add_argument("--json", action="store_true")

    hosts_parser = subparsers.add_parser("hosts", help="List or search hosts in one client")
    hosts_parser.add_argument("client")
    hosts_parser.add_argument("query", nargs="?")
    hosts_parser.add_argument("--host")
    hosts_parser.add_argument("--id", dest="systemframe_id")
    hosts_parser.add_argument("--ip", dest="addr_ip")
    hosts_parser.add_argument("--limit", type=int, default=20)
    hosts_parser.add_argument("--cursor")
    hosts_parser.add_argument("--json", action="store_true")

    connect_parser = subparsers.add_parser("connect", help="Resolve identifiers and connect if unique")
    connect_parser.add_argument("identifiers", nargs="*")
    connect_parser.add_argument("--client")
    connect_parser.add_argument("--host")
    connect_parser.add_argument("--id", dest="systemframe_id")
    connect_parser.add_argument("--ip", dest="addr_ip")
    connect_parser.add_argument("--context", dest="context_name")
    connect_parser.add_argument("--json", action="store_true")

    status_parser = subparsers.add_parser("status", help="Show active contexts and tunnels")
    status_parser.add_argument("--json", action="store_true")

    subparsers.add_parser("single", help=argparse.SUPPRESS)
    subparsers.add_parser("multi", help=argparse.SUPPRESS)
    return parser


def main(argv: list[str] | None = None) -> int:
    load_dotenv()
    parser = build_parser()
    args = parser.parse_args(argv)
    command = args.command

    _configure_logging()

    try:
        if command in (None, "connect"):
            return run_connect(
                args
                if command == "connect"
                else argparse.Namespace(
                    identifiers=[],
                    client=None,
                    host=None,
                    systemframe_id=None,
                    addr_ip=None,
                    context_name=None,
                    json=False,
                )
            )
        if command == "clients":
            return run_clients(args)
        if command == "hosts":
            return run_hosts(args)
        if command == "single":
            return _guided_connect()
        if command == "multi":
            return run_multi()
        if command == "status":
            return run_status(args)
    except NonInteractiveTerminalError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1
    except ValueError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 4

    parser.print_help()
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
