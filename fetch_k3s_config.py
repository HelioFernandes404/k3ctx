#!/usr/bin/env python3
"""
fetch_k3s_config.py

Usage:
    python3 fetch_k3s_config.py

This script:
1. Lists companies from inventory/ (Ansible-style inventories)
2. Lists hosts from the selected company
3. Shows VPN requirement for each host
4. Connects via SSH to the selected host
5. Fetches /etc/rancher/k3s/k3s.yaml
6. Replaces clusters[0].cluster.server with https://<internal-ip>:6443
7. Saves to ./{company}_{host}.yml

Dependencies:
    pip install paramiko pyyaml python-dotenv
"""

import os
import sys
import socket
import paramiko
from paramiko.proxy import ProxyCommand
from paramiko import SSHConfig, SSHClient
import yaml
from io import StringIO
from dotenv import load_dotenv
from pathlib import Path
import subprocess
import hashlib

# Import local modules
from src.inventory import load_inventories, extract_hosts_from_inventory, update_inventory_repo
from src.network import is_private_network, check_vpn_requirement, check_network_requirement
from src.ssh import load_ssh_config, make_ssh_client, get_internal_ip, fetch_remote_file_cached
from src.kubeconfig import update_kubeconfig_server, merge_kubeconfig
from src.tunnel import (
    get_unique_port, get_tunnel_pid_file, is_tunnel_running,
    kill_tunnel, kill_all_tunnels, create_tunnel, save_tunnel_pid
)
from src.cli import (
    NonInteractiveTerminalError,
    confirm_action,
    select_company,
    select_host,
)
from src.logging_config import setup_logging, get_logger

# Load environment variables from .env file
load_dotenv()

# Load config from project's config.yaml
CONFIG_FILE = os.getenv("CONFIG_FILE", str(Path(__file__).parent / "config.yaml"))
from src.config import load_config, get_config_value, resolve_inventory_path
config = load_config(os.path.expanduser(CONFIG_FILE))

# Configuration from config file + environment variables with defaults
REMOTE_PATH = get_config_value(config, 'remote_k3s_config_path', "/etc/rancher/k3s/k3s.yaml")
DEFAULT_KEY = os.path.expanduser(get_config_value(config, 'ssh_key_path', "~/.ssh/id_ed25519"))
TARGET_PORT = int(get_config_value(config, 'k3s_api_port', 6443))
PORT_RANGE_START = int(get_config_value(config, 'port_range_start', 16443))
PORT_RANGE_SIZE = int(get_config_value(config, 'port_range_size', 10000))
SSH_CONFIG_PATH = os.path.expanduser("~/.ssh/config")

INVENTORY_PATH = resolve_inventory_path(config, Path(__file__).parent)

TUNNEL_STATE_DIR = Path.home() / ".local" / "state" / "k9s-tunnels"
CACHE_DIR = Path.home() / ".cache" / "k9s-config"


def fetch_and_merge_kubeconfig(
    company: str,
    host_alias: str,
    host_info: dict,
    ssh_config: dict,
    remote_path: str,
    target_port: int,
    port_range_start: int,
    port_range_size: int,
    ssh_client: SSHClient = None
) -> tuple[str, int, str, str]:
    """
    Fetch kubeconfig from remote host and merge into local config.

    Args:
        company: Company name
        host_alias: SSH host alias
        host_info: Host info from inventory
        ssh_config: SSH configuration
        remote_path: Remote kubeconfig path
        target_port: Remote K3s API port
        port_range_start: Port range start
        port_range_size: Port range size
        ssh_client: Optional pre-connected SSH client (for testing)

    Returns:
        tuple: (context_name, local_port, internal_ip, new_content)

    Raises:
        RuntimeError: On SSH or fetch failure
    """
    logger = get_logger()

    # Track if we created the SSH client
    created_client = ssh_client is None

    # Create SSH connection if not provided
    if not ssh_client:
        hostname = ssh_config.get("hostname", host_alias)
        username = ssh_config.get("user", "ubuntu")
        port = int(ssh_config.get("port", 22))
        identity_files = ssh_config.get("identityfile")
        proxycmd = ssh_config.get("proxycommand")

        keyfile = None
        if identity_files:
            keyfile = os.path.expanduser(identity_files[0])
        elif os.path.exists(DEFAULT_KEY):
            keyfile = DEFAULT_KEY

        ssh_client = make_ssh_client(hostname, username, keyfile, port, proxycmd)

    try:
        # Get internal IP
        internal_ip = get_internal_ip(ssh_client)
        logger.debug(f"Detected internal IP for {host_alias}")

        # Define context name and cache path
        context_name = f"{company}-{host_alias}"
        cache_path = CACHE_DIR / f"{context_name}.yml"

        # Fetch kubeconfig with caching
        content, was_cached = fetch_remote_file_cached(
            ssh_client, remote_path, cache_path
        )

        # Generate unique port
        local_port = get_unique_port(context_name, port_range_start, port_range_size)

        # Update kubeconfig
        new_content = update_kubeconfig_server(
            content, internal_ip, target_port,
            use_localhost=True, local_port=local_port
        )

        # Merge into ~/.kube/config
        merge_kubeconfig(new_content, context_name)

        return context_name, local_port, internal_ip, new_content, was_cached
    finally:
        # Only close if we created the client
        if created_client and ssh_client:
            ssh_client.close()


def main():
    log_file_path = os.path.expanduser(os.getenv("K9S_LOG_FILE", "~/.local/state/k9s/k9s-config.log"))
    logger = setup_logging(log_file=log_file_path)
    logger.info("Starting k9s-config fetcher")
    logger.debug(f"Using inventory path: {INVENTORY_PATH}")

    # Update inventory repository before loading
    if INVENTORY_PATH.exists():
        print("Updating inventory repository...")
        success, message = update_inventory_repo(INVENTORY_PATH)
        if success:
            print(f"✓ {message}")
        else:
            print(f"⚠️  {message} (continuing with local version)")

    try:
        # Outer loop: company selection
        while True:
            company, inv_data = select_company(INVENTORY_PATH)
            if company is None:  # ESC on company selection
                print("Cancelled.")
                sys.exit(0)

            logger.debug(f"Selected company={company}")

            # Inner loop: host selection
            while True:
                host_alias, host_info = select_host(company, inv_data)
                if host_alias is None:  # ESC on host selection
                    break  # Back to company selection

                logger.debug(f"Selected host={host_alias}")

                # Check VPN requirement
                group = host_info["group"]
                needs_vpn = check_vpn_requirement(inv_data, group, host_alias)
                network_type, network_range = check_network_requirement(host_alias, host_info)

                # Show warnings for network requirements
                if needs_vpn:
                    print(f"\n⚠️  WARNING: This host requires VPN (argocd_use_socks5_proxy=true)")
                    print(f"   Make sure your VPN is connected before proceeding.")
                    try:
                        confirmed = confirm_action("Continue?", default=False)
                        if not confirmed or confirmed is None:
                            continue  # Back to host selection
                    except KeyboardInterrupt:
                        continue

                if network_type == "sshuttle":
                    print(f"\n🔒 NETWORK REQUIREMENT: This host is on private network {network_range}")
                    print(f"   You need to run sshuttle to access this network.")
                    print(f"\n   Example command:")
                    print(f"   sshuttle -v -r helio@100.64.5.10 {network_range}")
                    print(f"\n   Make sure sshuttle is running before proceeding.")
                    try:
                        confirmed = confirm_action("Continue?", default=False)
                        if not confirmed or confirmed is None:
                            continue  # Back to host selection
                    except KeyboardInterrupt:
                        continue

                # Set LOCAL_OUT based on company and host
                LOCAL_OUT = f"./{company}_{host_alias}.yml"

                print(f"\nReading SSH config (~/.ssh/config) for host alias '{host_alias}'...")
                cfg = load_ssh_config(host_alias, SSH_CONFIG_PATH)

                # Priority: ansible_host from inventory > hostname from SSH config > host_alias
                inventory_host = host_info.get("config", {}).get("ansible_host")
                if inventory_host:
                    hostname = inventory_host
                    cfg["hostname"] = hostname  # Update cfg so fetch_and_merge_kubeconfig uses it
                    print(f"Using ansible_host from inventory: {hostname}")
                else:
                    hostname = cfg.get("hostname", host_alias)

                username = cfg.get("user", "ubuntu")
                port = int(cfg.get("port", 22))
                identity_files = cfg.get("identityfile")
                proxycmd = cfg.get("proxycommand")

                keyfile = None
                if identity_files:
                    # identityfile entries may be relative or multiple; use first and expand
                    keyfile = os.path.expanduser(identity_files[0])
                else:
                    # if agent has keys, paramiko can try those; but prefer explicit fallback
                    if os.path.exists(DEFAULT_KEY):
                        keyfile = DEFAULT_KEY
                    else:
                        keyfile = None

                # Log connection details (without exposing full key path)
                key_status = 'configured' if keyfile else 'none'
                print(
                    f"Resolved: hostname={hostname} user={username} port={port} key={key_status} proxycmd={'yes' if proxycmd else 'no'}"
                )

                try:
                    print("Connected — detecting remote internal IP...")
                    # Fetch and merge kubeconfig
                    context_name, local_port, internal_ip, new_content, was_cached = fetch_and_merge_kubeconfig(
                        company=company,
                        host_alias=host_alias,
                        host_info=host_info,
                        ssh_config=cfg,
                        remote_path=REMOTE_PATH,
                        target_port=TARGET_PORT,
                        port_range_start=PORT_RANGE_START,
                        port_range_size=PORT_RANGE_SIZE
                    )

                    # Don't log the actual IP for security reasons
                    print("Internal IP detected successfully")
                    if was_cached:
                        print(f"✓ Using cached kubeconfig (unchanged on remote)")
                    else:
                        print(f"Fetching remote kubeconfig: {REMOTE_PATH}")
                    print("Updating server: field in kubeconfig...")
                    print(f"Configuring kubeconfig to use SSH tunnel (localhost:{local_port})")
                    print(f"Merging into ~/.kube/config as context '{context_name}'...")

                    kubeconfig_path = Path.home() / ".kube" / "config"
                    print(f"✓ Context '{context_name}' added to {kubeconfig_path}")
                    print(f"✓ Set as current context")

                    # Also save standalone file for backup
                    with open(LOCAL_OUT, "w") as f:
                        f.write(new_content)
                    print(f"✓ Standalone backup saved to {LOCAL_OUT}")

                    # Setup SSH tunnel
                    print(f"\nSetting up SSH tunnel...")
                    if is_tunnel_running(context_name):
                        print(f"✓ Tunnel already running for {context_name}")
                    else:
                        # Use hostname (from inventory) instead of alias (from ssh config)
                        ssh_target = f"{username}@{hostname}"
                        print(f"Creating tunnel: {ssh_target} -> localhost:{local_port} -> {internal_ip}:6443")
                        try:
                            pid = create_tunnel(ssh_target, internal_ip, local_port, TARGET_PORT)
                            save_tunnel_pid(context_name, pid)
                            print(f"✓ SSH tunnel created (PID: {pid})")
                        except Exception as e:
                            print(f"⚠️  Failed to create tunnel: {e}")
                            print(f"   You'll need to create it manually:")
                            print(f"   ssh -f -N -L {local_port}:{internal_ip}:{TARGET_PORT} {ssh_target}")

                    print(f"\nYou can now use kubectl/k9s directly!")
                    print(f"  kubectl get nodes")
                    print(f"  k9s -l debug")
                    print(f"\nTo switch contexts later:")
                    print(f"  kubectl config use-context {context_name}")
                    print(f"\nTo view/manage tunnels:")
                    print(f"  ls ~/.local/state/k9s-tunnels/")

                    if needs_vpn:
                        print("\n⚠️  Remember: This context requires VPN to access the cluster.")

                    if network_type == "sshuttle":
                        print(f"\n🔒 Remember: Keep sshuttle running to access this cluster.")
                        print(f"   sshuttle -v -r helio@100.64.5.10 {network_range}")

                    # Success - exit the program
                    return

                except Exception as e:
                    print(f"Failed to fetch and merge kubeconfig: {e}", file=sys.stderr)
                    # Ask if user wants to try another host
                    try:
                        retry = confirm_action("Try another host?", default=True)
                        if retry:
                            continue  # Back to host selection
                        else:
                            sys.exit(2)
                    except KeyboardInterrupt:
                        sys.exit(2)
    except NonInteractiveTerminalError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
