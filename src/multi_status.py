"""
Status display for multi-cluster connections.

Shows status of all connected clusters, active tunnels, and network requirements.
"""

import os
import subprocess
from pathlib import Path
from typing import List, Dict, Any, Optional
from .tunnel import is_tunnel_running, get_tunnel_pid_file, TUNNEL_STATE_DIR
from .network_validator import get_network_metadata
from .logging_config import get_logger

logger = get_logger()


def get_current_context() -> Optional[str]:
    """
    Get current kubectl context from kubeconfig.

    Returns:
        str|None: Current context name or None if not set
    """
    try:
        result = subprocess.run(
            ["kubectl", "config", "current-context"],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except (subprocess.TimeoutExpired, FileNotFoundError):
        pass
    return None


def get_tunnel_pid(context_name: str, state_dir: Optional[Path] = None) -> Optional[int]:
    """
    Get PID of running tunnel for context.

    Args:
        context_name: Kubernetes context name
        state_dir: Custom state directory

    Returns:
        int|None: PID if tunnel is running, None otherwise
    """
    if state_dir is None:
        state_dir = TUNNEL_STATE_DIR

    pid_file = get_tunnel_pid_file(context_name, state_dir)
    if not pid_file.exists():
        return None

    try:
        with open(pid_file) as f:
            pid = int(f.read().strip())
        # Verify process is still running
        os.kill(pid, 0)
        return pid
    except (ValueError, ProcessLookupError, OSError):
        return None


def get_tunnel_port(context_name: str) -> Optional[int]:
    """
    Extract local port from tunnel process.

    Args:
        context_name: Kubernetes context name

    Returns:
        int|None: Local port number or None if not found
    """
    from .tunnel import get_unique_port
    # Port is deterministic based on context name
    return get_unique_port(context_name)


def list_all_contexts(state_dir: Optional[Path] = None) -> List[Dict[str, Any]]:
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

    if not state_dir.exists():
        return []

    contexts = []

    # Find all .pid files
    for pid_file in state_dir.glob("*.pid"):
        context_name = pid_file.stem
        tunnel_running = is_tunnel_running(context_name, state_dir)
        pid = get_tunnel_pid(context_name, state_dir) if tunnel_running else None
        port = get_tunnel_port(context_name)
        network_meta = get_network_metadata(context_name, state_dir)

        contexts.append({
            'name': context_name,
            'tunnel_running': tunnel_running,
            'tunnel_pid': pid,
            'local_port': port,
            'network_metadata': network_meta
        })

    # Sort by name
    contexts.sort(key=lambda x: x['name'])
    return contexts


def show_status(state_dir: Optional[Path] = None) -> None:
    """
    Display formatted status of all clusters.

    Args:
        state_dir: Custom state directory
    """
    # ANSI color codes
    GREEN = '\033[0;32m'
    RED = '\033[0;31m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'  # No Color

    contexts = list_all_contexts(state_dir)
    current_context = get_current_context()

    if not contexts:
        print(f"{YELLOW}No connected clusters found.{NC}")
        print(f"\nRun: make run         # Connect single cluster")
        print(f"     make multi-connect  # Connect multiple clusters")
        return

    print(f"{GREEN}Connected clusters:{NC}")

    # Collect network requirements
    network_requirements = []

    for ctx in contexts:
        name = ctx['name']
        is_current = (name == current_context)
        current_marker = " (active)" if is_current else ""

        if ctx['tunnel_running']:
            port = ctx['local_port']
            pid = ctx['tunnel_pid']
            status_icon = f"{GREEN}✓{NC}"

            # Check network metadata
            network_meta = ctx['network_metadata']
            network_warning = ""
            if network_meta:
                network_type = network_meta.get('network_type')
                if network_type == 'sshuttle':
                    network_warning = f" {YELLOW}⚠ requires sshuttle{NC}"
                    network_requirements.append(network_meta)
                elif network_meta.get('needs_vpn'):
                    network_warning = f" {YELLOW}⚠ requires VPN{NC}"

            print(f"  {status_icon} {name} (localhost:{port}) [PID: {pid}]{network_warning}{current_marker}")
        else:
            print(f"  {RED}✗{NC} {name} (tunnel down){current_marker}")

    # Show current context
    if current_context:
        print(f"\n{GREEN}Current context:{NC} {current_context}")
    else:
        print(f"\n{YELLOW}No current context set{NC}")

    # Show active network requirements
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
