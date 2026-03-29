"""
SSH utilities for k9s-config.

Handles SSH connections, configuration loading, remote command execution,
and file transfers via SFTP.
"""

import os
import time
import hashlib
from pathlib import Path
from paramiko import SSHConfig, SSHClient
from paramiko.proxy import ProxyCommand
import paramiko
from typing import Dict, Any, Optional, Union, List
from .logging_config import get_logger

logger = get_logger()


def load_ssh_config(alias: str, ssh_config_path: Optional[str] = None) -> Dict[str, Any]:
    """
    Load SSH configuration for a given host alias.

    Args:
        alias: SSH host alias to lookup
        ssh_config_path: Path to SSH config file (default: ~/.ssh/config)

    Returns:
        dict: SSH configuration parameters
    """
    if ssh_config_path is None:
        ssh_config_path = os.path.expanduser("~/.ssh/config")

    logger.debug(f"Loading SSH config from: {ssh_config_path}")
    logger.debug(f"Looking up host alias: {alias}")

    cfg: Dict[str, Any] = {}
    if not os.path.exists(ssh_config_path):
        logger.warning(f"SSH config file not found: {ssh_config_path}")
        return cfg

    with open(ssh_config_path) as f:
        sc = SSHConfig()
        sc.parse(f)
        cfg = dict(sc.lookup(alias))

    logger.debug(f"SSH config resolved: hostname={cfg.get('hostname', 'N/A')}, user={cfg.get('user', 'N/A')}, port={cfg.get('port', 'N/A')}")

    return cfg


def choose_first(lst: Any, default: Any = None) -> Any:
    """
    Get first element from list or return default.

    Args:
        lst: List-like object or single value
        default: Default value if list is empty

    Returns:
        First element or default
    """
    if not lst:
        return default
    if isinstance(lst, (list, tuple)):
        return lst[0]
    return lst


def make_ssh_client(
    hostname: str,
    username: str,
    key_filename: Optional[str],
    port: int,
    proxycmd: Optional[str] = None,
    timeout: int = 10,
    max_retries: int = 3
) -> SSHClient:
    """
    Create and connect an SSH client with retry logic.

    Retries connection with exponential backoff on transient failures
    (connection timeouts, refused connections, temporary errors).

    Args:
        hostname: SSH hostname to connect to
        username: SSH username
        key_filename: Path to SSH private key (optional)
        port: SSH port
        proxycmd: ProxyCommand string for jump hosts (optional)
        timeout: Connection timeout in seconds
        max_retries: Maximum number of connection attempts (default: 3)

    Returns:
        SSHClient: Connected SSH client

    Raises:
        Exception: On connection failure after all retries
    """
    client = SSHClient()
    client.load_system_host_keys()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    connect_kwargs = dict(
        username=username,
        port=port,
        timeout=timeout,
        look_for_keys=True,
        allow_agent=True,
    )

    # key_filename may be None
    if key_filename:
        connect_kwargs["key_filename"] = key_filename

    if proxycmd:
        # paramiko proxy expects a socket-like ProxyCommand
        connect_kwargs["sock"] = ProxyCommand(proxycmd)

    # Retry with exponential backoff
    for attempt in range(1, max_retries + 1):
        try:
            logger.debug(f"SSH connection attempt {attempt}/{max_retries}")
            logger.debug(f"  hostname: {hostname}")
            logger.debug(f"  username: {username}")
            logger.debug(f"  port: {port}")
            logger.debug(f"  key_filename: {key_filename}")
            logger.debug(f"  proxycmd: {proxycmd}")
            logger.debug(f"  timeout: {timeout}s")

            client.connect(hostname, **connect_kwargs)
            logger.info(f"✓ SSH connection successful to {username}@{hostname}:{port}")
            return client
        except (paramiko.ssh_exception.NoValidConnectionsError,
                paramiko.ssh_exception.SSHException,
                OSError,
                TimeoutError) as e:
            if attempt == max_retries:
                logger.error(f"✗ SSH connection failed after {max_retries} attempts")
                logger.error(f"  Target: {username}@{hostname}:{port}")
                logger.error(f"  Error: {e}")
                logger.error(f"  Error type: {type(e).__name__}")
                raise

            # Exponential backoff: 1s, 2s, 4s
            wait_time = 2 ** (attempt - 1)
            logger.warning(f"SSH connection failed (attempt {attempt}/{max_retries}): {e}")
            logger.debug(f"Retrying in {wait_time}s...")
            time.sleep(wait_time)


def get_internal_ip(ssh: SSHClient) -> str:
    """
    Detect internal IPv4 address of remote host.

    Tries multiple commands to find the primary non-loopback IPv4 address.

    Args:
        ssh: Connected SSHClient instance

    Returns:
        str: Internal IPv4 address

    Raises:
        RuntimeError: If no valid internal IP is found
    """
    # Try a sequence of commands; return first non-loopback IPv4 found
    cmds = [
        "ip -4 addr show scope global | awk '/inet /{print $2}' | cut -d/ -f1 | head -n1",
        "hostname -I | awk '{print $1}'",
        "ip route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if($i==\"src\") print $(i+1)}' | head -n1",
    ]

    for cmd in cmds:
        stdin, stdout, stderr = ssh.exec_command(cmd)
        out = stdout.read().decode().strip()
        if out:
            # if command returned multiple IPs, take first token
            ip = out.split()[0]
            if ip and not ip.startswith("127.") and "." in ip:
                return str(ip)

    raise RuntimeError(
        "Could not detect internal IPv4 on remote host using tried commands."
    )


def fetch_remote_file(ssh: SSHClient, path: str, max_retries: int = 2) -> str:
    """
    Fetch file contents from remote host via SFTP with retry logic.

    Retries SFTP operations on transient failures (temporary connection issues,
    partial reads, etc).

    Args:
        ssh: Connected SSHClient instance
        path: Remote file path to read
        max_retries: Maximum number of fetch attempts (default: 2)

    Returns:
        str: File contents as string

    Raises:
        Exception: On SFTP or file read failure after all retries
    """
    for attempt in range(1, max_retries + 1):
        try:
            logger.debug(f"SFTP fetch attempt {attempt}/{max_retries}: {path}")
            sftp = ssh.open_sftp()
            try:
                with sftp.open(path, "r") as f:
                    data = f.read().decode()
                logger.debug(f"SFTP fetch successful on attempt {attempt}: {path}")
                return str(data)
            finally:
                sftp.close()
        except (OSError, IOError, paramiko.ssh_exception.SSHException) as e:
            if attempt == max_retries:
                logger.error(f"SFTP fetch failed after {max_retries} attempts: {path}: {e}")
                raise

            wait_time = 2 ** (attempt - 1)
            logger.warning(f"SFTP fetch failed (attempt {attempt}): {e}. Retrying in {wait_time}s...")
            time.sleep(wait_time)


def get_remote_file_hash(ssh: SSHClient, path: str) -> str:
    """
    Calculate SHA256 hash of remote file without downloading it.

    Args:
        ssh: Connected SSHClient instance
        path: Remote file path

    Returns:
        str: SHA256 hash of file contents

    Raises:
        RuntimeError: If remote hash calculation fails
    """
    # Try sha256sum first (most common), fallback to shasum -a 256
    commands = [
        f"sha256sum {path} 2>/dev/null | awk '{{print $1}}'",
        f"shasum -a 256 {path} 2>/dev/null | awk '{{print $1}}'",
    ]

    for cmd in commands:
        try:
            stdin, stdout, stderr = ssh.exec_command(cmd)
            hash_output = stdout.read().decode().strip()
            if hash_output and len(hash_output) == 64:  # SHA256 is 64 hex chars
                logger.debug(f"Remote file hash: {hash_output[:16]}...")
                return hash_output
        except Exception as e:
            logger.debug(f"Hash command failed: {cmd}: {e}")
            continue

    raise RuntimeError(f"Could not calculate remote file hash for {path}")


def get_local_file_hash(file_path: Path) -> Optional[str]:
    """
    Calculate SHA256 hash of local file.

    Args:
        file_path: Path to local file

    Returns:
        str: SHA256 hash or None if file doesn't exist
    """
    if not file_path.exists():
        return None

    try:
        sha256 = hashlib.sha256()
        with open(file_path, 'rb') as f:
            for chunk in iter(lambda: f.read(4096), b''):
                sha256.update(chunk)
        hash_str = sha256.hexdigest()
        logger.debug(f"Local file hash: {hash_str[:16]}...")
        return hash_str
    except Exception as e:
        logger.warning(f"Failed to calculate local file hash: {e}")
        return None


def fetch_remote_file_cached(
    ssh: SSHClient,
    remote_path: str,
    cache_path: Path,
    max_retries: int = 2
) -> tuple[str, bool]:
    """
    Fetch remote file with hash-based caching.

    Compares remote file hash with cached file hash. Only downloads if different.

    Args:
        ssh: Connected SSHClient instance
        remote_path: Remote file path to read
        cache_path: Local cache file path
        max_retries: Maximum number of fetch attempts

    Returns:
        tuple[str, bool]: (file_contents, was_cached)
            - file_contents: File contents as string
            - was_cached: True if served from cache, False if downloaded

    Raises:
        Exception: On hash calculation or fetch failure
    """
    # Get remote file hash
    try:
        remote_hash = get_remote_file_hash(ssh, remote_path)
    except Exception as e:
        logger.warning(f"Could not get remote hash, will download file: {e}")
        # Fallback to direct fetch if hash fails
        content = fetch_remote_file(ssh, remote_path, max_retries)
        return content, False

    # Get local cache hash
    local_hash = get_local_file_hash(cache_path)

    # Compare hashes
    if local_hash and local_hash == remote_hash:
        logger.info(f"Cache hit! Using cached kubeconfig (hash: {remote_hash[:16]}...)")
        with open(cache_path, 'r') as f:
            return f.read(), True

    # Cache miss - download file
    logger.info(f"Cache miss. Downloading kubeconfig (remote hash: {remote_hash[:16]}...)")
    content = fetch_remote_file(ssh, remote_path, max_retries)

    # Update cache
    cache_path.parent.mkdir(parents=True, exist_ok=True)
    with open(cache_path, 'w') as f:
        f.write(content)
    logger.debug(f"Updated cache: {cache_path}")

    return content, False
