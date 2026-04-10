"""Tunnel-related application use cases."""

from __future__ import annotations

from src.application.ports import TunnelManager


def kill_tunnel(context_name: str, tunnel_manager: TunnelManager) -> None:
    tunnel_manager.kill_tunnel(context_name)
