"""Tests for the local tunnel manager adapter."""

from __future__ import annotations

import pytest
from pytest_mock import MockerFixture

from src.infrastructure.adapters.tunnel_manager import LocalTunnelManager


def test_kill_tunnel_delegates_to_kill_tunnel_impl(mocker: MockerFixture) -> None:
    kill_impl = mocker.patch(
        "src.infrastructure.adapters.tunnel_manager.kill_tunnel_impl"
    )

    LocalTunnelManager().kill_tunnel("acme-prod")

    kill_impl.assert_called_once_with("acme-prod")


def test_kill_tunnel_passes_exact_context_name(mocker: MockerFixture) -> None:
    kill_impl = mocker.patch(
        "src.infrastructure.adapters.tunnel_manager.kill_tunnel_impl"
    )

    LocalTunnelManager().kill_tunnel("beta-staging")

    kill_impl.assert_called_once_with("beta-staging")


def test_kill_tunnel_propagates_os_error(mocker: MockerFixture) -> None:
    mocker.patch(
        "src.infrastructure.adapters.tunnel_manager.kill_tunnel_impl",
        side_effect=OSError("no such process"),
    )

    with pytest.raises(OSError, match="no such process"):
        LocalTunnelManager().kill_tunnel("acme-prod")
