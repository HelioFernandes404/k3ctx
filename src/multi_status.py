"""Status display for multi-cluster connections."""

from pathlib import Path
from typing import Any, Dict, List, Optional

from .models import EffectiveConfig
from .logging_config import get_logger
from .network_validator import get_network_metadata, validate_context_network_details
from .status_runtime import (
    get_current_context,
    get_tunnel_pid,
    get_tunnel_port,
    list_all_context_names,
    load_status_config,
)
from .tunnel import TUNNEL_STATE_DIR, is_tunnel_running

logger = get_logger()
ContextStatus = Dict[str, Any]
GREEN = '\033[0;32m'
RED = '\033[0;31m'
YELLOW = '\033[1;33m'
NC = '\033[0m'


def _build_context_status(
    context_name: str,
    config: EffectiveConfig,
    state_dir: Path,
) -> ContextStatus:
    tunnel_running = is_tunnel_running(context_name, state_dir)
    return {
        'name': context_name,
        'tunnel_running': tunnel_running,
        'tunnel_pid': get_tunnel_pid(context_name, state_dir) if tunnel_running else None,
        'local_port': get_tunnel_port(context_name, config),
        'network_metadata': get_network_metadata(context_name, state_dir),
        'network_validation': validate_context_network_details(context_name, state_dir),
    }


def _format_network_warning(
    network_meta: Dict[str, Any],
    network_validation: Dict[str, Any],
) -> str:
    warning_message = str(network_validation.get('warning') or "").lower()

    if "could not be read safely" in warning_message:
        return f" {YELLOW}⚠ network metadata unreadable{NC}"
    if not network_validation.get('ok', True):
        if network_meta.get('needs_vpn') or "vpn" in warning_message:
            return f" {YELLOW}⚠ requires VPN{NC}"
        if network_meta.get('network_type') == 'sshuttle' or "sshuttle" in warning_message:
            return f" {YELLOW}⚠ requires sshuttle{NC}"

    return ""


def list_all_contexts(state_dir: Optional[Path] = None) -> List[ContextStatus]:
    """
    List all configured contexts with tunnel status.

    Args:
        state_dir: Custom state directory

    Returns:
        list: List of dicts with context info
            [
                {
                    'name': 'company-host',
                    'tunnel_running': True,
                    'tunnel_pid': 12345,
                    'local_port': 16443,
                    'network_metadata': {...}
                }
            ]
    """
    if state_dir is None:
        state_dir = TUNNEL_STATE_DIR

    config = load_status_config()
    contexts: List[ContextStatus] = []

    for context_name in list_all_context_names(state_dir):
        contexts.append(_build_context_status(context_name, config, state_dir))

    # Sort by name
    contexts.sort(key=lambda context: str(context["name"]))
    return contexts


def show_status(state_dir: Optional[Path] = None) -> None:
    """
    Display formatted status of all clusters.

    Args:
        state_dir: Custom state directory
    """
    contexts = list_all_contexts(state_dir)
    current_context = get_current_context()

    if not contexts:
        print(f"{YELLOW}No connected clusters found.{NC}")
        print(f"\nRun: make run         # Connect single cluster")
        print(f"     make multi-connect  # Connect multiple clusters")
        return

    print(f"{GREEN}Connected clusters:{NC}")

    network_requirements: list[dict[str, Any]] = []

    for ctx in contexts:
        name = ctx['name']
        is_current = (name == current_context)
        current_marker = " (active)" if is_current else ""
        network_validation = ctx.get('network_validation') or {}
        network_meta = ctx.get('network_metadata') or {}
        network_warning = _format_network_warning(network_meta, network_validation)

        if (
            not network_validation.get('ok', True)
            and network_meta.get('network_type') == 'sshuttle'
        ):
            network_requirements.append(network_meta)

        if ctx['tunnel_running']:
            port = ctx['local_port']
            pid = ctx['tunnel_pid']
            status_icon = f"{GREEN}✓{NC}"

            print(f"  {status_icon} {name} (localhost:{port}) [PID: {pid}]{network_warning}{current_marker}")
        else:
            print(f"  {RED}✗{NC} {name} (tunnel down){network_warning}{current_marker}")

    if current_context:
        print(f"\n{GREEN}Current context:{NC} {current_context}")
    else:
        print(f"\n{YELLOW}No current context set{NC}")

    if network_requirements:
        print(f"\n{YELLOW}Active network requirements:{NC}")
        shown_commands = set()
        for meta in network_requirements:
            cmd = meta.get('sshuttle_command')
            if cmd and cmd not in shown_commands:
                print(f"  🔒 {cmd}")
                shown_commands.add(cmd)

    print(f"\n{GREEN}Commands:{NC}")
    print(f"  kubectl config use-context <name>  # Switch context")
    print(f"  make k9s                           # Launch k9s")
    print(f"  make tunnel-list                   # List tunnels")


if __name__ == "__main__":
    show_status()
