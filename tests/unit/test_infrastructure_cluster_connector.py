"""Unit tests for the local cluster connector adapter."""

from __future__ import annotations

from pathlib import Path
from unittest.mock import MagicMock

import pytest
from pytest_mock import MockerFixture

from src.application.ports import ClusterConnectionError
from src.domain.models import ClusterTarget, EffectiveConfig, NetworkRequirement
from src.infrastructure.adapters.cluster_connector import (
    LocalClusterConnector,
    _wait_for_kubernetes_api,
)


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


def test_connector_aborts_before_merge_when_api_readiness_check_fails(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    connector = LocalClusterConnector()
    config = build_config(tmp_path)
    target = build_target()
    ssh_client = MagicMock()

    mocker.patch(
        "src.infrastructure.adapters.cluster_connector.load_ssh_config",
        return_value={"hostname": "acme-prod"},
    )
    mocker.patch(
        "src.infrastructure.adapters.cluster_connector.resolve_ssh_connection_target",
        return_value=("resolved.example.internal", "ec2-user", "/tmp/test-key", 22, None),
    )
    mocker.patch(
        "src.infrastructure.adapters.cluster_connector.make_ssh_client",
        return_value=ssh_client,
    )
    mocker.patch(
        "src.infrastructure.adapters.cluster_connector._prepare_local_kubeconfig",
        return_value=("10.0.0.10", 16443, False, "apiVersion: v1\n"),
    )
    mocker.patch(
        "src.infrastructure.adapters.cluster_connector._ensure_tunnel",
        return_value=(4242, False),
    )
    wait_for_api = mocker.patch(
        "src.infrastructure.adapters.cluster_connector._wait_for_kubernetes_api",
        side_effect=ClusterConnectionError(
            code="kubernetes_api_unreachable",
            message="Kubernetes API did not become ready on https://127.0.0.1:16443",
        ),
    )
    kill_tunnel = mocker.patch("src.infrastructure.adapters.cluster_connector.kill_tunnel")
    merge_kubeconfig = mocker.patch(
        "src.infrastructure.adapters.cluster_connector.merge_kubeconfig",
    )

    with pytest.raises(ClusterConnectionError):
        connector.connect(target, config, NetworkRequirement.none())

    wait_for_api.assert_called_once_with(
        16443,
        kubeconfig_text="apiVersion: v1\n",
        timeout_seconds=15.0,
        poll_interval_seconds=0.25,
    )
    kill_tunnel.assert_called_once_with("acme-prod")
    merge_kubeconfig.assert_not_called()
    ssh_client.close.assert_called_once()


def test_connector_recreates_stale_reused_tunnel_after_readiness_failure(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    connector = LocalClusterConnector()
    config = build_config(tmp_path)
    target = build_target()
    ssh_client = MagicMock()

    mocker.patch(
        "src.infrastructure.adapters.cluster_connector.load_ssh_config",
        return_value={"hostname": "acme-prod"},
    )
    mocker.patch(
        "src.infrastructure.adapters.cluster_connector.resolve_ssh_connection_target",
        return_value=("resolved.example.internal", "ec2-user", "/tmp/test-key", 22, None),
    )
    mocker.patch(
        "src.infrastructure.adapters.cluster_connector.make_ssh_client",
        return_value=ssh_client,
    )
    mocker.patch(
        "src.infrastructure.adapters.cluster_connector._prepare_local_kubeconfig",
        return_value=("10.0.0.10", 16443, False, "apiVersion: v1\n"),
    )
    ensure_tunnel = mocker.patch(
        "src.infrastructure.adapters.cluster_connector._ensure_tunnel",
        side_effect=[(4242, True), (4343, False)],
    )
    wait_for_api = mocker.patch(
        "src.infrastructure.adapters.cluster_connector._wait_for_kubernetes_api",
        side_effect=[
            ClusterConnectionError(
                code="kubernetes_api_unreachable",
                message="Kubernetes API did not become ready on https://127.0.0.1:16443",
            ),
            None,
        ],
    )
    kill_tunnel = mocker.patch("src.infrastructure.adapters.cluster_connector.kill_tunnel")
    merge_kubeconfig = mocker.patch(
        "src.infrastructure.adapters.cluster_connector.merge_kubeconfig",
    )
    mocker.patch("src.infrastructure.adapters.cluster_connector.save_network_metadata")

    result = connector.connect(target, config, NetworkRequirement.none())

    assert result.tunnel_pid == 4343
    assert ensure_tunnel.call_count == 2
    assert wait_for_api.call_count == 2
    kill_tunnel.assert_called_once_with("acme-prod")
    merge_kubeconfig.assert_called_once_with("apiVersion: v1\n", "acme-prod")
    ssh_client.close.assert_called_once()


def test_wait_for_kubernetes_api_allows_slow_tls_handshake_within_budget(
    mocker: MockerFixture,
) -> None:
    """Per-request timeout must not be clamped to the small poll interval.

    Regression: with poll_interval_seconds=0.25, each urlopen used to inherit
    the same 0.25 s timeout, so a handshake taking ~0.4 s always timed out and
    the whole deadline elapsed without a single successful call. The fix
    raises the per-request floor to 2.0 s while keeping the 0.25 s poll
    cadence.
    """

    response = MagicMock()
    response.status = 200
    response.__enter__ = lambda self: self
    response.__exit__ = lambda self, exc_type, exc, tb: None

    observed_timeouts: list[float] = []

    def fake_urlopen(request: object, *, timeout: float, context: object) -> object:
        observed_timeouts.append(timeout)
        return response

    mocker.patch(
        "src.infrastructure.adapters.cluster_connector.urllib.request.urlopen",
        side_effect=fake_urlopen,
    )

    _wait_for_kubernetes_api(
        16443,
        kubeconfig_text="apiVersion: v1\n",
        timeout_seconds=5.0,
        poll_interval_seconds=0.25,
    )

    assert observed_timeouts, "urlopen must be called at least once"
    assert observed_timeouts[0] >= 2.0, (
        f"per-request timeout too tight: {observed_timeouts[0]}"
    )
