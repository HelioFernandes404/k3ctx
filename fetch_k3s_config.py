#!/usr/bin/env python3
"""Legacy wrapper for the official single-cluster CLI command."""

from __future__ import annotations

from src.domain.network import (
    check_network_requirement,
    check_vpn_requirement,
    is_private_network,
)
from src.interfaces.cli.app import main as cli_main
from src.tunnel import get_unique_port


def main() -> int:
    return cli_main(["single"])


if __name__ == "__main__":
    raise SystemExit(main())


__all__ = [
    "check_network_requirement",
    "check_vpn_requirement",
    "get_unique_port",
    "is_private_network",
    "main",
]
