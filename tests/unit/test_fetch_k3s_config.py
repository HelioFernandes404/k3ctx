"""Unit tests for the legacy single-cluster wrapper."""

from __future__ import annotations

from pytest_mock import MockerFixture

import fetch_k3s_config


def test_main_delegates_to_official_cli_single_command(
    mocker: MockerFixture,
) -> None:
    cli_main = mocker.patch("fetch_k3s_config.cli_main", return_value=0)

    assert fetch_k3s_config.main() == 0
    cli_main.assert_called_once_with(["single"])


def test_wrapper_reexports_network_helpers() -> None:
    assert fetch_k3s_config.check_vpn_requirement is not None
    assert fetch_k3s_config.check_network_requirement is not None
    assert fetch_k3s_config.is_private_network("10.0.0.1") is True
    assert isinstance(fetch_k3s_config.get_unique_port("acme-prod"), int)
