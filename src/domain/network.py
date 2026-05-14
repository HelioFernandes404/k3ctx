"""Pure network requirement policies."""

from __future__ import annotations

import ipaddress
from typing import Any, Dict, Mapping, Optional, Tuple


_VPN_FLAG = "k3s_use_socks5_proxy"
_VPN_FLAG_LEGACY = "argocd_use_socks5_proxy"
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
    try:
        ip = ipaddress.ip_address(ip_or_hostname)
        return ip.is_private
    except ValueError:
        return False


def check_vpn_requirement(inv_data: Any, group_name: str, host_name: str) -> bool:
    del host_name
    vars_dict = _extract_group_vars(inv_data, group_name)
    return bool(vars_dict.get(_VPN_FLAG, False)) or bool(vars_dict.get(_VPN_FLAG_LEGACY, False))


def check_network_requirement(hostname: str, host_info: Dict[str, Any]) -> Tuple[Optional[str], Optional[str]]:
    del hostname
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
    host_info = {"config": dict(host_config)}
    network_type, network_range = check_network_requirement(group_name or "", host_info)

    needs_vpn = bool(host_config.get(_VPN_FLAG, False)) or bool(host_config.get(_VPN_FLAG_LEGACY, False))
    vars_dict = host_config.get("vars")
    if isinstance(vars_dict, Mapping):
        needs_vpn = needs_vpn or bool(vars_dict.get(_VPN_FLAG, False)) or bool(vars_dict.get(_VPN_FLAG_LEGACY, False))
    if isinstance(group_vars, Mapping):
        needs_vpn = needs_vpn or bool(group_vars.get(_VPN_FLAG, False)) or bool(group_vars.get(_VPN_FLAG_LEGACY, False))

    return network_type, network_range, needs_vpn
