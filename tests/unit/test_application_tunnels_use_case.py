"""Tests for tunnel application use cases."""

from __future__ import annotations

from src.application.use_cases.tunnels import kill_tunnel


class StubTunnelManager:
    def __init__(self) -> None:
        self.killed: list[str] = []

    def kill_tunnel(self, context_name: str) -> None:
        self.killed.append(context_name)


def test_kill_tunnel_delegates_to_manager() -> None:
    manager = StubTunnelManager()

    kill_tunnel("acme-prod", manager)

    assert manager.killed == ["acme-prod"]


def test_kill_tunnel_passes_exact_context_name() -> None:
    manager = StubTunnelManager()

    kill_tunnel("beta-staging", manager)

    assert manager.killed == ["beta-staging"]


def test_kill_tunnel_calls_manager_once() -> None:
    manager = StubTunnelManager()

    kill_tunnel("acme-prod", manager)

    assert len(manager.killed) == 1
