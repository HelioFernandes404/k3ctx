"""
SSH tunnel management for k9s-config.

Handles creation, lifecycle, and PID file management for SSH tunnels
used to access K3s clusters.
"""

import os
import hashlib
import subprocess
import time
from pathlib import Path
from typing import Optional
from .logging_config import get_logger

logger = get_logger()


# Default tunnel state directory
TUNNEL_STATE_DIR = Path.home() / ".local" / "state" / "k9s-tunnels"


def get_unique_port(
    context_name: str,
    port_range_start: int = 16443,
    port_range_size: int = 10000
) -> int:
    """
    Generate a unique port for a context (deterministic based on name).

    Uses MD5 hash of context name to generate a port within specified range.
    Default range is 16443-26443 (10000 ports).

    Args:
        context_name: Kubernetes context name (e.g., "company-host")
        port_range_start: Starting port number (default: 16443)
        port_range_size: Number of ports in range (default: 10000)

    Returns:
        int: Port number within specified range

    Example:
        # Default range (16443-26443)
        port = get_unique_port("my-context")

        # Custom range (20000-25000)
        port = get_unique_port("my-context", port_range_start=20000, port_range_size=5000)
    """
    # Use hash to generate deterministic port within range
    hash_int = int(hashlib.md5(context_name.encode()).hexdigest()[:4], 16)
    return port_range_start + (hash_int % port_range_size)


def get_tunnel_pid_file(context_name: str, state_dir: Optional[Path] = None) -> Path:
    """
    Get the PID file path for a tunnel.

    Args:
        context_name: Kubernetes context name
        state_dir: Custom state directory (default: TUNNEL_STATE_DIR)

    Returns:
        Path: Path to PID file
    """
    if state_dir is None:
        state_dir = TUNNEL_STATE_DIR

    state_dir.mkdir(parents=True, exist_ok=True)
    return state_dir / f"{context_name}.pid"


def is_tunnel_running(context_name: str, state_dir: Optional[Path] = None) -> bool:
    """
    Check if tunnel for this context is already running.

    Args:
        context_name: Kubernetes context name
        state_dir: Custom state directory (default: TUNNEL_STATE_DIR)

    Returns:
        bool: True if tunnel is running, False otherwise
    """
    pid_file = get_tunnel_pid_file(context_name, state_dir)
    if not pid_file.exists():
        return False

    try:
        with open(pid_file) as f:
            pid = int(f.read().strip())
        # Check if process is still running
        os.kill(pid, 0)
        return True
    except (ValueError, ProcessLookupError, OSError):
        # PID file is stale
        pid_file.unlink(missing_ok=True)
        return False


def kill_tunnel(context_name: str, state_dir: Optional[Path] = None) -> None:
    """
    Kill SSH tunnel for a context.

    Args:
        context_name: Kubernetes context name
        state_dir: Custom state directory (default: TUNNEL_STATE_DIR)
    """
    pid_file = get_tunnel_pid_file(context_name, state_dir)
    if not pid_file.exists():
        return

    try:
        with open(pid_file) as f:
            pid = int(f.read().strip())
        os.kill(pid, 15)  # SIGTERM
        logger.info(f"Killed existing tunnel for {context_name} (PID {pid})")
    except (ValueError, ProcessLookupError, OSError):
        pass
    finally:
        pid_file.unlink(missing_ok=True)


def kill_all_tunnels(state_dir: Optional[Path] = None) -> None:
    """
    Kill all k9s SSH tunnels.

    Args:
        state_dir: Custom state directory (default: TUNNEL_STATE_DIR)
    """
    if state_dir is None:
        state_dir = TUNNEL_STATE_DIR

    if not state_dir.exists():
        return

    for pid_file in state_dir.glob("*.pid"):
        context_name = pid_file.stem
        kill_tunnel(context_name, state_dir)


def create_tunnel(ssh_host: str, internal_ip: str, local_port: int, remote_port: int = 6443) -> Optional[int]:
    """
    Create SSH tunnel in background and return PID.

    Args:
        ssh_host: SSH host alias (from ~/.ssh/config)
        internal_ip: Internal IP of the K3s server
        local_port: Local port to listen on
        remote_port: Remote K3s API port (default: 6443)

    Returns:
        int|None: PID of tunnel process, or None if couldn't determine

    Raises:
        RuntimeError: If tunnel creation fails
    """
    cmd = [
        "ssh", "-f", "-N",
        "-o", "ExitOnForwardFailure=yes",
        "-o", "ServerAliveInterval=60",
        "-L", f"{local_port}:{internal_ip}:{remote_port}",
        ssh_host
    ]

    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        raise RuntimeError(f"Failed to create SSH tunnel: {result.stderr}")

    # Find the PID of the SSH tunnel we just created
    # Give it a moment to establish
    time.sleep(0.5)

    find_pid = subprocess.run(
        ["pgrep", "-f", f"ssh.*{local_port}:{internal_ip}:{remote_port}"],
        capture_output=True,
        text=True
    )

    if find_pid.returncode == 0 and find_pid.stdout.strip():
        return int(find_pid.stdout.strip().split()[0])

    # Fallback: assume it worked, we'll verify later
    return None


def save_tunnel_pid(context_name: str, pid: Optional[int], state_dir: Optional[Path] = None) -> None:
    """
    Save tunnel PID to file.

    Args:
        context_name: Kubernetes context name
        pid: Process ID of tunnel
        state_dir: Custom state directory (default: TUNNEL_STATE_DIR)
    """
    if pid:
        pid_file = get_tunnel_pid_file(context_name, state_dir)
        with open(pid_file, 'w') as f:
            f.write(str(pid))


def get_network_metadata_file(context_name: str, state_dir: Optional[Path] = None) -> Path:
    """
    Get the network metadata file path for a context.

    Args:
        context_name: Kubernetes context name
        state_dir: Custom state directory (default: TUNNEL_STATE_DIR)

    Returns:
        Path: Path to network metadata file
    """
    if state_dir is None:
        state_dir = TUNNEL_STATE_DIR

    state_dir.mkdir(parents=True, exist_ok=True)
    return state_dir / f"{context_name}.network"


def save_network_metadata(
    context_name: str,
    network_type: Optional[str] = None,
    network_range: Optional[str] = None,
    sshuttle_command: Optional[str] = None,
    needs_vpn: bool = False,
    internal_ip: Optional[str] = None,
    state_dir: Optional[Path] = None
) -> None:
    """
    Save network metadata for a context.

    Args:
        context_name: Kubernetes context name
        network_type: Type of network requirement ('sshuttle', 'vpn', 'direct')
        network_range: Network range (e.g., '192.168.90.0/24')
        sshuttle_command: Full sshuttle command to run
        needs_vpn: Whether VPN is required
        internal_ip: Internal IP of the cluster
        state_dir: Custom state directory (default: TUNNEL_STATE_DIR)
    """
    # Only save if there are actual network requirements
    if not network_type and not needs_vpn:
        return

    metadata = {
        'network_type': network_type,
        'network_range': network_range,
        'sshuttle_command': sshuttle_command,
        'needs_vpn': needs_vpn,
        'internal_ip': internal_ip
    }

    # Remove None values
    metadata = {k: v for k, v in metadata.items() if v is not None}

    network_file = get_network_metadata_file(context_name, state_dir)

    try:
        import yaml
        with open(network_file, 'w') as f:
            yaml.safe_dump(metadata, f, default_flow_style=False)
        logger.debug(f"Saved network metadata for {context_name}")
    except Exception as e:
        logger.warning(f"Failed to save network metadata for {context_name}: {e}")


def remove_network_metadata(context_name: str, state_dir: Optional[Path] = None) -> None:
    """
    Remove network metadata file for a context.

    Args:
        context_name: Kubernetes context name
        state_dir: Custom state directory (default: TUNNEL_STATE_DIR)
    """
    network_file = get_network_metadata_file(context_name, state_dir)
    network_file.unlink(missing_ok=True)
