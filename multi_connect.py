#!/usr/bin/env python3
"""
multi_connect.py

Connect to multiple K3s clusters simultaneously.

Usage:
    python3 multi_connect.py

This script:
1. Lists all available clusters from inventory
2. Allows multi-select via checkbox interface
3. Validates network requirements (VPN/sshuttle)
4. Connects to each cluster sequentially
5. Sets first cluster as active context
"""

import os
import sys
import subprocess
from pathlib import Path
from typing import List, Dict, Any, Tuple, Optional
import questionary
from questionary import Style
from dotenv import load_dotenv

# Import local modules
from src.inventory import load_inventories, extract_hosts_from_inventory
from src.network import check_vpn_requirement, check_network_requirement
from src.ssh import load_ssh_config
from src.tunnel import (
    is_tunnel_running, create_tunnel, save_tunnel_pid,
    save_network_metadata, TUNNEL_STATE_DIR
)
from src.logging_config import setup_logging, get_logger
from fetch_k3s_config import fetch_and_merge_kubeconfig

# Load environment variables
load_dotenv()

# Load config
from src.config import load_config, get_config_value, resolve_inventory_path
CONFIG_FILE = os.getenv("CONFIG_FILE", str(Path(__file__).parent / "config.yaml"))
config = load_config(os.path.expanduser(CONFIG_FILE))

# Configuration
REMOTE_PATH = get_config_value(config, 'remote_k3s_config_path', "/etc/rancher/k3s/k3s.yaml")
DEFAULT_KEY = os.path.expanduser(get_config_value(config, 'ssh_key_path', "~/.ssh/id_ed25519"))
TARGET_PORT = int(get_config_value(config, 'k3s_api_port', 6443))
PORT_RANGE_START = int(get_config_value(config, 'port_range_start', 16443))
PORT_RANGE_SIZE = int(get_config_value(config, 'port_range_size', 10000))
SSH_CONFIG_PATH = os.path.expanduser("~/.ssh/config")

INVENTORY_PATH = resolve_inventory_path(config, Path(__file__).parent)

# Custom style for questionary
custom_style = Style([
    ('qmark', 'fg:#E91E63 bold'),
    ('question', 'bold'),
    ('answer', 'fg:#2196F3 bold'),
    ('pointer', 'fg:#E91E63 bold'),
    ('highlighted', 'fg:#E91E63 bold'),
    ('selected', 'fg:#E91E63'),
    ('separator', 'fg:#cc5454'),
    ('instruction', ''),
    ('text', ''),
    ('disabled', 'fg:#858585 italic'),
    ('checkbox', 'fg:#E91E63 bold'),
    ('checkbox-selected', 'fg:#E91E63'),
])


def build_cluster_list(inventory_path: Path) -> List[Dict[str, Any]]:
    """
    Build a flat list of all available clusters.

    Args:
        inventory_path: Path to inventory directory

    Returns:
        list: List of cluster info dicts
            [
                {
                    'label': 'company: host [indicators]',
                    'value': 'company:host',
                    'company': 'company',
                    'host_alias': 'host',
                    'host_info': {...},
                    'inv_data': {...},
                    'needs_vpn': bool,
                    'network_type': str|None,
                    'network_range': str|None
                }
            ]
    """
    inventories = load_inventories(inventory_path)
    clusters = []

    for company, inv_data in sorted(inventories.items()):
        hosts = extract_hosts_from_inventory(inv_data)

        for host_alias in sorted(hosts.keys()):
            host_info = hosts[host_alias]
            group = host_info["group"]

            # Check network requirements
            needs_vpn = check_vpn_requirement(inv_data, group, host_alias)
            network_type, network_range = check_network_requirement(host_alias, host_info)

            # Build label with indicators
            indicators = []
            if needs_vpn:
                indicators.append("[VPN]")
            if network_type == "sshuttle":
                indicators.append(f"[sshuttle]")

            label = f"{company}: {host_alias}"
            if indicators:
                label += " " + " ".join(indicators)

            clusters.append({
                'label': label,
                'value': f"{company}:{host_alias}",
                'company': company,
                'host_alias': host_alias,
                'host_info': host_info,
                'inv_data': inv_data,
                'needs_vpn': needs_vpn,
                'network_type': network_type,
                'network_range': network_range
            })

    return clusters


def show_network_warnings(selected_clusters: List[Dict[str, Any]]) -> bool:
    """
    Show warnings for clusters with network requirements and get confirmation.

    Args:
        selected_clusters: List of cluster info dicts

    Returns:
        bool: True if user confirms to continue, False otherwise
    """
    # Group clusters by network requirement
    vpn_required = []
    sshuttle_required = []
    direct = []

    for cluster in selected_clusters:
        if cluster['needs_vpn']:
            vpn_required.append(cluster)
        elif cluster['network_type'] == 'sshuttle':
            sshuttle_required.append(cluster)
        else:
            direct.append(cluster)

    # Show summary
    print("\n" + "="*60)
    print("Selected clusters:")
    print("="*60)

    if direct:
        print("\n✓ Direct access (no special setup):")
        for c in direct:
            print(f"  • {c['company']}: {c['host_alias']}")

    if sshuttle_required:
        print("\n⚠ Requires sshuttle:")
        for c in sshuttle_required:
            print(f"  • {c['company']}: {c['host_alias']} → {c['network_range']}")

    if vpn_required:
        print("\n⚠ Requires VPN:")
        for c in vpn_required:
            print(f"  • {c['company']}: {c['host_alias']}")

    # Show required commands
    if sshuttle_required or vpn_required:
        print("\n" + "="*60)
        print("Network setup commands:")
        print("="*60)

        if sshuttle_required:
            # Get unique network ranges
            ranges = set(c['network_range'] for c in sshuttle_required if c['network_range'])
            for network_range in sorted(ranges):
                print(f"\n🔒 For {network_range}:")
                print(f"   sshuttle -v -r helio@100.64.5.10 {network_range}")

        if vpn_required:
            print("\n🔐 Ensure VPN connection is active before proceeding")

    print("\n" + "="*60)

    # Ask for confirmation
    try:
        confirmed = questionary.confirm(
            "Continue with multi-cluster connection?",
            default=True,
            style=custom_style
        ).ask()
        return confirmed if confirmed is not None else False
    except KeyboardInterrupt:
        return False


def connect_cluster(cluster: Dict[str, Any], logger) -> Dict[str, Any]:
    """
    Connect to a single cluster.

    Args:
        cluster: Cluster info dict
        logger: Logger instance

    Returns:
        dict: Connection result
            {
                'success': bool,
                'context_name': str,
                'local_port': int,
                'internal_ip': str,
                'tunnel_pid': int,
                'error': str|None
            }
    """
    company = cluster['company']
    host_alias = cluster['host_alias']
    host_info = cluster['host_info']
    context_name = f"{company}-{host_alias}"

    result = {
        'success': False,
        'context_name': context_name,
        'local_port': None,
        'internal_ip': None,
        'tunnel_pid': None,
        'error': None,
        'cluster': cluster
    }

    try:
        # Load SSH config
        ssh_config = load_ssh_config(host_alias, SSH_CONFIG_PATH)

        # Fetch and merge kubeconfig
        logger.info(f"Connecting to {context_name}...")
        print(f"\n📡 Connecting to {context_name}...")

        context_name, local_port, internal_ip, new_content, was_cached = fetch_and_merge_kubeconfig(
            company=company,
            host_alias=host_alias,
            host_info=host_info,
            ssh_config=ssh_config,
            remote_path=REMOTE_PATH,
            target_port=TARGET_PORT,
            port_range_start=PORT_RANGE_START,
            port_range_size=PORT_RANGE_SIZE
        )

        result['local_port'] = local_port
        result['internal_ip'] = internal_ip

        if was_cached:
            print(f"   ✓ Using cached kubeconfig")
        else:
            print(f"   ✓ Fetched kubeconfig from remote")

        # Setup tunnel
        if is_tunnel_running(context_name):
            print(f"   ✓ Tunnel already running")
            # Get existing PID
            pid_file = TUNNEL_STATE_DIR / f"{context_name}.pid"
            if pid_file.exists():
                with open(pid_file) as f:
                    result['tunnel_pid'] = int(f.read().strip())
        else:
            print(f"   Creating tunnel: localhost:{local_port} → {internal_ip}:6443")
            pid = create_tunnel(host_alias, internal_ip, local_port, TARGET_PORT)
            save_tunnel_pid(context_name, pid)
            result['tunnel_pid'] = pid
            print(f"   ✓ Tunnel created (PID: {pid})")

        # Save network metadata
        if cluster['network_type'] or cluster['needs_vpn']:
            sshuttle_cmd = None
            if cluster['network_type'] == 'sshuttle':
                sshuttle_cmd = f"sshuttle -v -r helio@100.64.5.10 {cluster['network_range']}"

            save_network_metadata(
                context_name=context_name,
                network_type=cluster['network_type'],
                network_range=cluster['network_range'],
                sshuttle_command=sshuttle_cmd,
                needs_vpn=cluster['needs_vpn'],
                internal_ip=internal_ip
            )

        print(f"   ✓ Context '{context_name}' configured")
        result['success'] = True

    except Exception as e:
        error_msg = str(e)
        logger.error(f"Failed to connect to {context_name}: {error_msg}")
        print(f"   ✗ Failed: {error_msg}")
        result['error'] = error_msg

    return result


def set_current_context(context_name: str) -> bool:
    """
    Set kubectl current context.

    Args:
        context_name: Context name to set

    Returns:
        bool: True if successful
    """
    try:
        subprocess.run(
            ["kubectl", "config", "use-context", context_name],
            check=True,
            capture_output=True
        )
        return True
    except subprocess.CalledProcessError:
        return False


def select_clusters_interactive(clusters: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    """
    Interactive multi-selection using autocomplete.

    Args:
        clusters: List of all available clusters

    Returns:
        list: Selected clusters
    """
    selected = []
    cluster_map = {c['label']: c for c in clusters}
    available_labels = [c['label'] for c in clusters]

    print(f"\n📋 Available: {len(available_labels)} clusters")
    print("💡 Tip: Type to search, Enter to add, Ctrl+C when done\n")

    while True:
        try:
            # Show current selection
            if selected:
                print(f"\n✓ Selected ({len(selected)}):")
                for i, c in enumerate(selected, 1):
                    print(f"  {i}. {c['label']}")
                print()

            # Filter out already selected clusters
            remaining = [label for label in available_labels if label not in [c['label'] for c in selected]]

            if not remaining:
                print("All clusters selected!")
                break

            # Ask for next cluster
            choice = questionary.autocomplete(
                f"Add cluster (type to search, {len(remaining)} remaining):",
                choices=remaining,
                match_middle=True,
                style=custom_style
            ).ask()

            if choice is None:  # ESC pressed
                break

            if choice in cluster_map:
                selected.append(cluster_map[choice])
                print(f"  ✓ Added: {choice}")

        except KeyboardInterrupt:
            print("\n")
            break

    return selected


def main():
    log_file_path = os.path.expanduser(os.getenv("K9S_LOG_FILE", "~/.local/state/k9s/k9s-config.log"))
    logger = setup_logging(log_file=log_file_path)
    logger.info("Starting multi-cluster connection")

    # Build cluster list
    print("Loading available clusters...")
    clusters = build_cluster_list(INVENTORY_PATH)

    if not clusters:
        print("No clusters found in inventory.", file=sys.stderr)
        sys.exit(1)

    print(f"Found {len(clusters)} clusters in inventory")

    # Interactive multi-selection
    try:
        selected = select_clusters_interactive(clusters)

        if not selected:
            print("\nNo clusters selected. Cancelled.")
            sys.exit(0)

    except KeyboardInterrupt:
        print("\nCancelled.")
        sys.exit(0)

    # Show network warnings and get confirmation
    if not show_network_warnings(selected):
        print("Cancelled.")
        sys.exit(0)

    # Connect to each cluster
    print("\n" + "="*60)
    print("Connecting to clusters...")
    print("="*60)

    results = []
    for cluster in selected:
        result = connect_cluster(cluster, logger)
        results.append(result)

    # Show summary
    successful = [r for r in results if r['success']]
    failed = [r for r in results if not r['success']]

    print("\n" + "="*60)
    print("Connection Summary")
    print("="*60)

    print(f"\n✓ Connected: {len(successful)}/{len(results)} clusters")
    for r in successful:
        network_note = ""
        c = r['cluster']
        if c['network_type'] == 'sshuttle':
            network_note = " ⚠ requires sshuttle"
        elif c['needs_vpn']:
            network_note = " ⚠ requires VPN"

        print(f"  ✓ {r['context_name']} (localhost:{r['local_port']}){network_note}")

    if failed:
        print(f"\n✗ Failed: {len(failed)} clusters")
        for r in failed:
            print(f"  ✗ {r['context_name']} - {r['error']}")

    if not successful:
        print("\nNo clusters connected successfully.")
        sys.exit(1)

    # Set first successful cluster as active context
    first_context = successful[0]['context_name']
    print(f"\nSetting active context to: {first_context}")
    if set_current_context(first_context):
        print(f"✓ Active context: {first_context}")
    else:
        print(f"⚠ Failed to set active context (you can set it manually)")

    # Show network requirements reminder
    network_reminders = []
    for r in successful:
        c = r['cluster']
        if c['network_type'] == 'sshuttle':
            sshuttle_cmd = f"sshuttle -v -r helio@100.64.5.10 {c['network_range']}"
            if sshuttle_cmd not in network_reminders:
                network_reminders.append(sshuttle_cmd)

    if network_reminders:
        print("\n" + "="*60)
        print("⚠ Active network requirements:")
        print("="*60)
        for cmd in network_reminders:
            print(f"  🔒 {cmd}")

    # Show usage instructions
    print("\n" + "="*60)
    print("Usage:")
    print("="*60)
    print("  kubectl config use-context <name>  # Switch between clusters")
    print("  make k9s                           # Launch k9s")
    print("  make status                        # Show connection status")
    print("  make tunnel-list                   # List active tunnels")


if __name__ == "__main__":
    main()
