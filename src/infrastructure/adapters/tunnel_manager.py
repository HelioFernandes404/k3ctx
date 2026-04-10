"""Tunnel management adapter."""

from __future__ import annotations

from src.tunnel import kill_tunnel as kill_tunnel_impl


class LocalTunnelManager:
    def kill_tunnel(self, context_name: str) -> None:
        kill_tunnel_impl(context_name)
