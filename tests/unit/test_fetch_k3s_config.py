"""Unit tests for manual single-cluster flow."""

from pathlib import Path
from typing import Any

import fetch_k3s_config
from _pytest.capture import CaptureFixture
from pytest_mock import MockerFixture
from src.models import (
    ClusterTarget,
    ConnectResult,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)


def build_config(tmp_path: Path) -> EffectiveConfig:
    return EffectiveConfig(
        inventory_path=tmp_path,
        ssh_config_path=str(tmp_path / "ssh_config"),
        ssh_key_path=str(tmp_path / "id_ed25519"),
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )


def build_host_info(
    *,
    ansible_host: str = "8.8.8.8",
    needs_vpn: bool = False,
) -> dict[str, Any]:
    return {
        "group": "k3s_cluster",
        "config": {"ansible_host": ansible_host},
        "group_vars": {"argocd_use_socks5_proxy": True} if needs_vpn else {},
    }


def build_success_result() -> ConnectResult:
    return ConnectResult(
        success=True,
        context_name="acme-prod",
        local_port=16443,
        internal_ip="10.0.0.10",
        tunnel_pid=4242,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
    )


def build_failure_result() -> ConnectResult:
    return ConnectResult(
        success=False,
        context_name="acme-prod",
        local_port=None,
        internal_ip=None,
        tunnel_pid=None,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
        error=OperationError(
            code="connect_failed",
            message="Cluster connection failed",
            detail="boom token=secret",
        ),
    )


def test_main_uses_connect_service_and_returns_zero_on_success(
    mocker: MockerFixture,
    tmp_path: Path,
    capsys: CaptureFixture[str],
) -> None:
    config = build_config(tmp_path)
    mocker.patch("fetch_k3s_config.load_effective_config", return_value=config)
    setup_logging = mocker.patch("fetch_k3s_config.setup_logging")
    mocker.patch("fetch_k3s_config.update_inventory_repo", return_value=(True, "updated"))
    mocker.patch("fetch_k3s_config.select_company", return_value=("acme", {"all": {}}))
    mocker.patch(
        "fetch_k3s_config.select_host",
        return_value=("prod", build_host_info()),
    )
    connect_cluster = mocker.patch(
        "fetch_k3s_config.connect_cluster",
        return_value=build_success_result(),
    )

    assert fetch_k3s_config.main() == 0

    connect_cluster.assert_called_once_with(
        target=ClusterTarget(
            company="acme",
            host_alias="prod",
            group="k3s_cluster",
            host_config={"ansible_host": "8.8.8.8"},
            group_vars={},
        ),
        config=config,
        allow_manual_network=True,
    )
    setup_logging.assert_called_once()
    assert setup_logging.call_args.kwargs["structured"] is True
    output = capsys.readouterr().out
    assert "acme-prod" in output
    assert "localhost:16443" in output
    assert "backup" not in output.lower()


def test_main_returns_zero_when_company_selection_is_cancelled(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    mocker.patch("fetch_k3s_config.load_effective_config", return_value=config)
    mocker.patch("fetch_k3s_config.setup_logging")
    mocker.patch("fetch_k3s_config.update_inventory_repo", return_value=(True, "updated"))
    mocker.patch("fetch_k3s_config.select_company", return_value=(None, None))
    connect_cluster = mocker.patch("fetch_k3s_config.connect_cluster")

    assert fetch_k3s_config.main() == 0
    connect_cluster.assert_not_called()


def test_main_retries_another_host_after_failed_connection(
    mocker: MockerFixture,
    tmp_path: Path,
    capsys: CaptureFixture[str],
) -> None:
    config = build_config(tmp_path)
    mocker.patch("fetch_k3s_config.load_effective_config", return_value=config)
    mocker.patch("fetch_k3s_config.setup_logging")
    mocker.patch("fetch_k3s_config.update_inventory_repo", return_value=(True, "updated"))
    mocker.patch("fetch_k3s_config.select_company", return_value=("acme", {"all": {}}))
    mocker.patch(
        "fetch_k3s_config.select_host",
        side_effect=[
            ("broken", build_host_info(ansible_host="1.1.1.1")),
            ("prod", build_host_info()),
        ],
    )
    mocker.patch("fetch_k3s_config.confirm_action", return_value=True)
    connect_cluster = mocker.patch(
        "fetch_k3s_config.connect_cluster",
        side_effect=[build_failure_result(), build_success_result()],
    )

    assert fetch_k3s_config.main() == 0

    assert connect_cluster.call_count == 2
    output = capsys.readouterr().out
    assert "Failed to connect" in output
    assert "acme-prod" in output


def test_main_returns_one_when_connection_fails_and_retry_is_declined(
    mocker: MockerFixture,
    tmp_path: Path,
    capsys: CaptureFixture[str],
) -> None:
    config = build_config(tmp_path)
    mocker.patch("fetch_k3s_config.load_effective_config", return_value=config)
    setup_logging = mocker.patch("fetch_k3s_config.setup_logging")
    mocker.patch("fetch_k3s_config.update_inventory_repo", return_value=(True, "updated"))
    mocker.patch("fetch_k3s_config.select_company", return_value=("acme", {"all": {}}))
    mocker.patch(
        "fetch_k3s_config.select_host",
        return_value=("prod", build_host_info()),
    )
    mocker.patch("fetch_k3s_config.confirm_action", return_value=False)
    mocker.patch(
        "fetch_k3s_config.connect_cluster",
        return_value=build_failure_result(),
    )

    assert fetch_k3s_config.main() == 1

    setup_logging.assert_called_once()
    assert setup_logging.call_args.kwargs["structured"] is True
    error_output = capsys.readouterr().out
    assert "Cluster connection failed" in error_output
    assert "token=secret" not in error_output


def test_main_returns_zero_when_retry_prompt_is_interrupted(
    mocker: MockerFixture,
    tmp_path: Path,
    capsys: CaptureFixture[str],
) -> None:
    config = build_config(tmp_path)
    mocker.patch("fetch_k3s_config.load_effective_config", return_value=config)
    setup_logging = mocker.patch("fetch_k3s_config.setup_logging")
    mocker.patch("fetch_k3s_config.update_inventory_repo", return_value=(True, "updated"))
    mocker.patch("fetch_k3s_config.select_company", return_value=("acme", {"all": {}}))
    mocker.patch(
        "fetch_k3s_config.select_host",
        return_value=("prod", build_host_info()),
    )
    mocker.patch(
        "fetch_k3s_config.connect_cluster",
        return_value=build_failure_result(),
    )
    mocker.patch("fetch_k3s_config.confirm_action", side_effect=KeyboardInterrupt)

    assert fetch_k3s_config.main() == 0

    setup_logging.assert_called_once()
    assert setup_logging.call_args.kwargs["structured"] is True
    error_output = capsys.readouterr().out
    assert "Cluster connection failed" in error_output
    assert "token=secret" not in error_output
