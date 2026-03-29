"""
Network utilities for k9s-config.

Handles private network detection, VPN requirement checks, and
sshuttle configuration detection.
"""

import ipaddress
from typing import Tuple, Optional, Dict, Any


def is_private_network(ip_or_hostname: str) -> bool:
    """
    Check if IP/hostname is in private network range.

    Args:
        ip_or_hostname: IP address or hostname string

    Returns:
        bool: True if private IP, False otherwise
    """
    # Try to parse as IP
    try:
        ip = ipaddress.ip_address(ip_or_hostname)
        return ip.is_private
    except ValueError:
        # Not a valid IP, might be hostname
        return False


def check_vpn_requirement(inv_data: Any, group_name: str, host_name: str) -> bool:
    """
    Check if a host requires VPN based on group vars.

    Args:
        inv_data: Inventory data dict
        group_name: Group name the host belongs to
        host_name: Host name (unused but kept for signature compatibility)

    Returns:
        bool: True if argocd_use_socks5_proxy is set in group vars
    """
    if not isinstance(inv_data, dict) or "all" not in inv_data:
        return False

    all_data = inv_data["all"]
    if "children" not in all_data:
        return False

    children = all_data["children"]
    if group_name not in children:
        return False

    group_data = children[group_name]
    if isinstance(group_data, dict) and "vars" in group_data:
        vars_dict = group_data["vars"]
        return bool(vars_dict.get("argocd_use_socks5_proxy", False))

    return False


def check_network_requirement(hostname: str, host_info: Dict[str, Any]) -> Tuple[Optional[str], Optional[str]]:
    """
    Check if host requires sshuttle/VPN based on IP range.

    Args:
        hostname: Hostname (unused but kept for signature compatibility)
        host_info: Host info dict with "config" key

    Returns:
        tuple: (network_type, network_range)
            - ("sshuttle", "192.168.1.0/24") for private IPs
            - (None, None) for public IPs
    """
    # Check if ansible_host is defined in host config
    config = host_info.get("config", {})
    ansible_host = config.get("ansible_host")

    if not ansible_host:
        return None, None

    # Check if it's a private IP
    if is_private_network(ansible_host):
        # Detect network range
        try:
            ip = ipaddress.ip_address(ansible_host)
            network = ipaddress.ip_network(f"{ip}/24", strict=False)
            return "sshuttle", str(network)
        except ValueError:
            return "network", None

    return None, None
