"""Tests for interface-agnostic cluster connection use cases."""

from __future__ import annotations

from pathlib import Path

from src.application.ports import ConnectionArtifacts
from src.application.use_cases.connect import connect_cluster, connect_multiple
from src.domain.models import ClusterTarget, EffectiveConfig


class StubConnector:
    def __init__(self, result: ConnectionArtifacts | Exception) -> None:
        self.result = result
        self.calls: list[tuple[ClusterTarget, EffectiveConfig]] = []

    def connect(
        self,
        target: ClusterTarget,
        config: EffectiveConfig,
        requirement: object,
    ) -> ConnectionArtifacts:
        self.calls.append((target, config))
        if isinstance(self.result, Exception):
            raise self.result
        return self.result


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


def build_target(ansible_host: str = "203.0.113.10") -> ClusterTarget:
    return ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": ansible_host},
        group_vars={},
    )


def test_connect_cluster_delegates_to_connector_and_returns_structured_result(
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = build_target()
    connector = StubConnector(
        ConnectionArtifacts(
            local_port=16443,
            internal_ip="10.0.0.10",
            tunnel_pid=4242,
            used_cache=False,
        )
    )

    result = connect_cluster(target=target, config=config, connector=connector)

    assert result.success is True
    assert result.context_name == "acme-prod"
    assert result.local_port == 16443
    assert result.internal_ip == "10.0.0.10"
    assert result.tunnel_pid == 4242
    assert result.error is None
    assert connector.calls == [(target, config)]


def test_connect_cluster_blocks_when_manual_network_setup_is_required(
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = build_target("10.0.0.10")
    connector = StubConnector(
        ConnectionArtifacts(
            local_port=16443,
            internal_ip="10.0.0.10",
            tunnel_pid=4242,
            used_cache=False,
        )
    )

    result = connect_cluster(
        target=target,
        config=config,
        connector=connector,
        allow_manual_network=False,
    )

    assert result.success is False
    assert result.error is not None
    assert result.error.code == "network_requirement_unmet"
    assert connector.calls == []


def test_connect_multiple_preserves_target_order(
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    first = build_target("203.0.113.10")
    second = ClusterTarget(
        company="beta",
        host_alias="staging",
        group="k3s_cluster",
        host_config={"ansible_host": "203.0.113.11"},
        group_vars={},
    )
    connector = StubConnector(
        ConnectionArtifacts(
            local_port=16443,
            internal_ip="10.0.0.10",
            tunnel_pid=4242,
            used_cache=False,
        )
    )

    results = connect_multiple(
        targets=[first, second],
        config=config,
        connector=connector,
    )

    assert [result.context_name for result in results] == ["acme-prod", "beta-staging"]
    assert connector.calls == [(first, config), (second, config)]
