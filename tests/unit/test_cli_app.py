"""Tests for the official CLI interface."""

from __future__ import annotations

from pathlib import Path

from pytest_mock import MockerFixture

from src.domain.models import ClusterTarget, ConnectResult, EffectiveConfig, NetworkRequirement
from src.interfaces.cli.app import main


def build_config(tmp_path: Path) -> EffectiveConfig:
    return EffectiveConfig(
        inventory_path=tmp_path / "inventory",
        ssh_config_path="~/.ssh/config",
        ssh_key_path="~/.ssh/id_ed25519",
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )


def build_target() -> ClusterTarget:
    return ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "203.0.113.10"},
        group_vars={},
    )


def build_success_result(context_name: str = "acme-prod") -> ConnectResult:
    return ConnectResult(
        success=True,
        context_name=context_name,
        local_port=16443,
        internal_ip="10.0.0.10",
        tunnel_pid=4242,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
    )


def test_single_command_uses_application_services(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = build_target()
    mocker.patch("src.interfaces.cli.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.cli.app.setup_logging")
    mocker.patch("src.interfaces.cli.app.refresh_inventory_if_possible")
    mocker.patch("src.interfaces.cli.app.list_cluster_targets", return_value=[target])
    mocker.patch("src.interfaces.cli.app.select_company", return_value="acme")
    mocker.patch("src.interfaces.cli.app.select_single_target", return_value=target)
    mocker.patch("src.interfaces.cli.app.show_manual_network_warnings", return_value=True)
    connect_cluster = mocker.patch(
        "src.interfaces.cli.app.connect_cluster",
        return_value=build_success_result(),
    )

    exit_code = main(["single"])

    assert exit_code == 0
    connect_cluster.assert_called_once()


def test_multi_command_sets_first_successful_context(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    first = build_target()
    second = ClusterTarget(
        company="beta",
        host_alias="staging",
        group="k3s_cluster",
        host_config={"ansible_host": "203.0.113.11"},
        group_vars={},
    )
    mocker.patch("src.interfaces.cli.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.cli.app.setup_logging")
    mocker.patch("src.interfaces.cli.app.refresh_inventory_if_possible")
    mocker.patch("src.interfaces.cli.app.list_cluster_targets", return_value=[first, second])
    mocker.patch("src.interfaces.cli.app.select_multiple_targets", return_value=[first, second])
    mocker.patch("src.interfaces.cli.app.show_network_warnings", return_value=True)
    mocker.patch(
        "src.interfaces.cli.app.connect_multiple",
        return_value=[build_success_result("acme-prod"), build_success_result("beta-staging")],
    )
    set_current_context = mocker.patch(
        "src.interfaces.cli.app.set_current_context",
        return_value=None,
    )

    exit_code = main(["multi"])

    assert exit_code == 0
    set_current_context.assert_called_once_with(
        "acme-prod",
        switcher=mocker.ANY,
        require_confirmation=False,
        confirmed=True,
    )
