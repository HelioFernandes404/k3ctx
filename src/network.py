"""Network utilities for k9s-config."""

import ipaddress
from typing import Any, Dict, Mapping, Optional, Tuple


_VPN_FLAG = "argocd_use_socks5_proxy"
_ANSIBLE_HOST = "ansible_host"


def _extract_group_vars(inv_data: Any, group_name: str) -> Mapping[str, Any]:
    if not isinstance(inv_data, dict):
        return {}

    all_data = inv_data.get("all")
    if not isinstance(all_data, dict):
        return {}

    children = all_data.get("children")
    if not isinstance(children, dict):
        return {}

    group_data = children.get(group_name)
    if not isinstance(group_data, dict):
        return {}

    vars_dict = group_data.get("vars")
    if not isinstance(vars_dict, Mapping):
        return {}

    return vars_dict


def _network_range_for_host(ansible_host: str) -> tuple[Optional[str], Optional[str]]:
    if not is_private_network(ansible_host):
        return None, None

    try:
        ip = ipaddress.ip_address(ansible_host)
        network = ipaddress.ip_network(f"{ip}/24", strict=False)
        return "sshuttle", str(network)
    except ValueError:
        return "network", None


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
    return bool(_extract_group_vars(inv_data, group_name).get(_VPN_FLAG, False))


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
    ansible_host = config.get(_ANSIBLE_HOST)

    if not ansible_host:
        return None, None

    return _network_range_for_host(str(ansible_host))


def detect_network_requirement(
    host_config: Mapping[str, Any] | Dict[str, Any],
    group_vars: Mapping[str, Any] | Dict[str, Any] | None = None,
    group_name: Optional[str] = None,
) -> tuple[Optional[str], Optional[str], bool]:
    """
    Detect manual network requirements without depending on CLI inventory flow.

    Accepts the flattened host config used by service-layer models and keeps the
    older `check_*` helpers available for existing callers.
    """
    host_info = {"config": dict(host_config)}
    network_type, network_range = check_network_requirement(group_name or "", host_info)

    needs_vpn = bool(host_config.get(_VPN_FLAG, False))
    vars_dict = host_config.get("vars")
    if isinstance(vars_dict, Mapping):
        needs_vpn = needs_vpn or bool(vars_dict.get(_VPN_FLAG, False))
    if isinstance(group_vars, Mapping):
        needs_vpn = needs_vpn or bool(group_vars.get(_VPN_FLAG, False))

    return network_type, network_range, needs_vpn
