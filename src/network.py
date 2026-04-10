"""Compatibility wrapper for pure network policies."""

from src.domain.network import (
    check_network_requirement,
    check_vpn_requirement,
    detect_network_requirement,
    is_private_network,
)

__all__ = [
    "check_network_requirement",
    "check_vpn_requirement",
    "detect_network_requirement",
    "is_private_network",
]
