"""
Network validation utilities for k9s multi-connect.

Validates that required network setup (sshuttle/VPN) is active
before using clusters.
"""

import subprocess
import socket
from typing import Optional, Dict, Any
from pathlib import Path
from .logging_config import get_logger

logger = get_logger()


def check_sshuttle_active(network_range: str) -> bool:
    """
    Check if sshuttle is routing traffic for a network range.

    Args:
        network_range: Network range (e.g., "192.168.90.0/24")

    Returns:
        bool: True if sshuttle appears to be routing this network
    """
    try:
        # Check if there's a sshuttle process running
        result = subprocess.run(
            ["pgrep", "-f", f"sshuttle.*{network_range}"],
            capture_output=True,
            text=True,
            timeout=2
        )
        if result.returncode == 0 and result.stdout.strip():
            logger.debug(f"Found sshuttle process for {network_range}")
            return True

        # Alternative: check for any sshuttle process
        result = subprocess.run(
            ["pgrep", "-f", "sshuttle"],
            capture_output=True,
            text=True,
            timeout=2
        )
        if result.returncode == 0 and result.stdout.strip():
            logger.debug("Found generic sshuttle process")
            return True

        return False
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False


def validate_network_access(internal_ip: str, timeout: int = 3) -> bool:
    """
    Validate that an internal IP is reachable.

    Args:
        internal_ip: IP address to test
        timeout: Connection timeout in seconds

    Returns:
        bool: True if IP is reachable
    """
    try:
        # Try to connect to common K3s API port
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(timeout)
        result = sock.connect_ex((internal_ip, 6443))
        sock.close()

        # Return code 0 means connection successful
        return result == 0
    except (socket.error, socket.timeout):
        return False


def get_network_metadata(context_name: str, state_dir: Optional[Path] = None) -> Optional[Dict[str, Any]]:
    """
    Load network metadata for a context.

    Args:
        context_name: Kubernetes context name
        state_dir: Custom state directory (default: ~/.local/state/k9s-tunnels)

    Returns:
        dict|None: Network metadata or None if file doesn't exist

    Example metadata:
        {
            'network_type': 'sshuttle',
            'network_range': '192.168.90.0/24',
            'sshuttle_command': 'sshuttle -v -r user@host 192.168.90.0/24',
            'internal_ip': '192.168.90.10'
        }
    """
    if state_dir is None:
        state_dir = Path.home() / ".local" / "state" / "k9s-tunnels"

    network_file = state_dir / f"{context_name}.network"
    if not network_file.exists():
        return None

    try:
        import yaml
        with open(network_file) as f:
            return yaml.safe_load(f)
    except Exception as e:
        logger.warning(f"Failed to load network metadata for {context_name}: {e}")
        return None


def validate_context_network(context_name: str, state_dir: Optional[Path] = None) -> tuple[bool, Optional[str]]:
    """
    Validate network requirements for a context.

    Args:
        context_name: Kubernetes context name
        state_dir: Custom state directory

    Returns:
        tuple: (is_valid, warning_message)
            - (True, None) if network is properly configured
            - (False, "warning message") if network setup is missing
    """
    metadata = get_network_metadata(context_name, state_dir)

    # No network requirements
    if not metadata:
        return True, None

    network_type = metadata.get('network_type')

    # VPN requirement - can't auto-detect, just warn
    if metadata.get('needs_vpn'):
        return False, "This cluster requires VPN connection"

    # sshuttle requirement
    if network_type == 'sshuttle':
        network_range = metadata.get('network_range')
        sshuttle_cmd = metadata.get('sshuttle_command', f'sshuttle -v -r <gateway> {network_range}')

        if not check_sshuttle_active(network_range):
            warning = (
                f"This cluster requires sshuttle for {network_range}\n"
                f"  Run: {sshuttle_cmd}"
            )
            return False, warning

    return True, None
