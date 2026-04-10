"""Official CLI entrypoint built on the shared application layer."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

from dotenv import load_dotenv

from src.application.use_cases.connect import connect_cluster, connect_multiple
from src.application.use_cases.discovery import (
    list_client_summaries,
    load_host_records,
    resolve_host,
    search_hosts,
)
from src.application.use_cases.inventory import (
    find_target_by_context_name,
    refresh_inventory_if_possible,
)
from src.application.use_cases.status import list_context_status, validate_context_network
from src.application.use_cases.tunnels import kill_tunnel as kill_tunnel_use_case
from src.bootstrap import ServiceContainer, build_service_container
from src.config import load_effective_config
from src.domain.discovery import HostQuery, HostRecord
from src.domain.models import EffectiveConfig
from src.interfaces.cli.presenters import (
    print_client_page,
    print_host_page,
    print_host_resolution_failure,
    print_single_failure,
    print_single_success,
    print_status,
)
from src.interfaces.serialization import (
    client_page_payload,
    host_page_payload,
    host_resolution_payload,
    to_jsonable,
)
from src.logging_config import setup_logging
from src.status_runtime import get_current_context, get_tunnel_pid

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


def _print_removed_command_error(command: str) -> int:
    print(
        f"Error: `{command}` has been removed. "
        f"Use `{_cli_name()} connect <identifier>` with explicit arguments.",
        file=sys.stderr,
    )
    return 4


def _default_config_dir() -> Path:
    override = os.getenv("K9S_CONFIG_DIR")
    return Path(override).expanduser() if override else Path.home() / ".k9s-config"


def _default_log_dir() -> Path:
    override = os.getenv("K9S_LOG_DIR")
    return Path(override).expanduser() if override else Path.home() / ".local" / "state" / "k9s"


def _copy_default_config(project_dir: Path, config_dir: Path) -> bool:
    example = project_dir / ".k9s-config-example" / "config.yaml"
    destination = config_dir / "config.yaml"
    if destination.exists() or not example.exists():
        return False
    shutil.copy2(example, destination)
    return True


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
        print(
            "Error: connect requires at least one identifier.",
            file=sys.stderr,
        )
        return 4

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

    result = connect_cluster(
        target=target,
        config=config,
        connector=services.connector,
        allow_manual_network=False,
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


def run_init(args: argparse.Namespace) -> int:
    del args
    config_dir = _default_config_dir()
    log_dir = _default_log_dir()

    config_dir.mkdir(parents=True, exist_ok=True)
    copied_default = _copy_default_config(PROJECT_DIR, config_dir)
    log_dir.mkdir(parents=True, exist_ok=True)

    print(f"Config directory: {config_dir}")
    if copied_default:
        print(f"Created default config: {config_dir / 'config.yaml'}")
    else:
        print(f"Config file ready: {config_dir / 'config.yaml'}")
    print(f"Log directory: {log_dir}")
    print(f"Next: {_cli_name()} connect <identifier>")
    return 0


def run_k9s(args: argparse.Namespace) -> int:
    del args
    _, services = _load_runtime()
    current_context = get_current_context()
    if not current_context:
        print("No current kubernetes context set.", file=sys.stderr)
        print(f"Run: {_cli_name()} connect <identifier>", file=sys.stderr)
        return 1

    tunnel_pid = get_tunnel_pid(current_context)
    if tunnel_pid is None:
        print(f"Tunnel not running for context '{current_context}'.", file=sys.stderr)
        print(f"Run: {_cli_name()} connect --context {current_context}", file=sys.stderr)
        return 1

    validation = validate_context_network(current_context, services.status_reader)
    if not bool(validation.get("ok", True)):
        warning = validation.get("warning")
        if warning:
            print(f"Warning: {warning}")
        network_metadata = validation.get("network_metadata")
        if isinstance(network_metadata, dict):
            sshuttle_command = network_metadata.get("sshuttle_command")
            if isinstance(sshuttle_command, str) and sshuttle_command:
                print(f"Run: {sshuttle_command}")

    try:
        result = subprocess.run(["k9s", "-l", "debug"], check=False)
    except FileNotFoundError:
        print("k9s executable not found in PATH.", file=sys.stderr)
        return 1
    return int(result.returncode)


def run_tunnel_list(args: argparse.Namespace) -> int:
    del args
    _, services = _load_runtime()
    items = list_context_status(services.status_reader)
    running = [item for item in items if bool(item.get("tunnel_running"))]

    print("Active SSH tunnels:")
    if not running:
        print("  (none)")
        return 0

    for item in running:
        print(f"  ✓ {item['name']} (PID: {item.get('tunnel_pid')})")
    return 0


def run_tunnel_kill(args: argparse.Namespace) -> int:
    _, services = _load_runtime()
    kill_tunnel_use_case(args.context_name, services.tunnel_manager)
    print(f"Killed tunnel for {args.context_name}")
    return 0


def run_tunnel_kill_all(args: argparse.Namespace) -> int:
    del args
    _, services = _load_runtime()
    items = list_context_status(services.status_reader)
    running_contexts = [str(item["name"]) for item in items if bool(item.get("tunnel_running"))]

    if not running_contexts:
        print("(none to kill)")
        return 0

    for context_name in running_contexts:
        kill_tunnel_use_case(context_name, services.tunnel_manager)
        print(f"Killed tunnel for {context_name}")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog=_cli_name())
    subparsers = parser.add_subparsers(dest="command")

    subparsers.add_parser("init", help="Prepare local config and log directories")

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

    subparsers.add_parser("k9s", help="Launch k9s after tunnel validation")

    status_parser = subparsers.add_parser("status", help="Show active contexts and tunnels")
    status_parser.add_argument("--json", action="store_true")

    subparsers.add_parser("tunnel-list", help="List active SSH tunnels")
    tunnel_kill_parser = subparsers.add_parser("tunnel-kill", help="Kill one tunnel by context")
    tunnel_kill_parser.add_argument("context_name")
    subparsers.add_parser("tunnel-kill-all", help="Kill all managed tunnels")

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
        if command == "init":
            return run_init(args)
        if command == "k9s":
            return run_k9s(args)
        if command == "single":
            return _print_removed_command_error("single")
        if command == "multi":
            return _print_removed_command_error("multi")
        if command == "status":
            return run_status(args)
        if command == "tunnel-list":
            return run_tunnel_list(args)
        if command == "tunnel-kill":
            return run_tunnel_kill(args)
        if command == "tunnel-kill-all":
            return run_tunnel_kill_all(args)
    except ValueError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 4

    parser.print_help()
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
