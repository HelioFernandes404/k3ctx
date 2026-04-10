#!/usr/bin/env python3
"""Legacy wrapper for the official multi-cluster CLI command."""

from __future__ import annotations

from src.interfaces.cli.app import main as cli_main
from src.interfaces.cli.presenters import (
    print_multi_summary as _print_summary,
    print_network_reminders as _print_network_reminders,
    show_network_warnings,
)
from src.interfaces.cli.prompts import format_multi_target_label as format_cluster_label


def main() -> int:
    return cli_main(["multi"])


if __name__ == "__main__":
    raise SystemExit(main())


__all__ = [
    "_print_network_reminders",
    "_print_summary",
    "format_cluster_label",
    "main",
    "show_network_warnings",
]
