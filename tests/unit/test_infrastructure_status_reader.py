"""Tests for the local status reader adapter."""

from __future__ import annotations

from pathlib import Path

from pytest_mock import MockerFixture

from src.domain.models import EffectiveConfig
from src.infrastructure.adapters.status_reader import LocalStatusReader
from src.tunnel import get_unique_port


def test_list_context_status_reads_tunnel_and_network_metadata(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch(
        "src.infrastructure.adapters.status_reader.get_current_context",
        return_value="acme-prod",
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.list_all_context_names",
        return_value=["acme-prod", "acme-dev"],
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.is_tunnel_running",
        side_effect=[True, False],
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.get_tunnel_pid",
        side_effect=[4242, None],
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.get_tunnel_port",
        side_effect=[16443, 16444],
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.get_network_metadata",
        side_effect=[
            {"network_type": "sshuttle", "network_range": "10.0.0.0/24"},
            None,
        ],
    )

    items = LocalStatusReader(tmp_path).list_context_status()

    assert items == [
        {
            "name": "acme-prod",
            "is_current": True,
            "tunnel_running": True,
            "tunnel_pid": 4242,
            "local_port": 16443,
            "network_metadata": {
                "network_type": "sshuttle",
                "network_range": "10.0.0.0/24",
            },
        },
        {
            "name": "acme-dev",
            "is_current": False,
            "tunnel_running": False,
            "tunnel_pid": None,
            "local_port": 16444,
            "network_metadata": None,
        },
    ]


def test_validate_context_network_returns_structured_details(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.infrastructure.adapters.status_reader.validate_context_network_details",
        return_value={
            "context_name": "acme-prod",
            "ok": False,
            "warning": "This cluster requires VPN connection",
            "network_metadata": {"needs_vpn": True},
        },
    )

    result = LocalStatusReader().validate_context_network("acme-prod")

    assert result["context_name"] == "acme-prod"
    assert result["ok"] is False
    assert result["network_metadata"] == {"needs_vpn": True}


def test_list_context_status_uses_canonical_port_range(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch(
        "src.infrastructure.adapters.status_reader.get_current_context",
        return_value=None,
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.list_all_context_names",
        return_value=["acme-prod"],
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.is_tunnel_running",
        return_value=True,
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.get_tunnel_pid",
        return_value=4242,
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.get_network_metadata",
        return_value=None,
    )
    mocker.patch(
        "src.infrastructure.adapters.status_reader.load_status_config",
        return_value=EffectiveConfig(
            inventory_path=tmp_path / "inventory",
            ssh_config_path="~/.ssh/config",
            ssh_key_path="~/.ssh/id_ed25519",
            remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
            k3s_api_port=6443,
            port_range_start=20000,
            port_range_size=5000,
        ),
    )

    items = LocalStatusReader(tmp_path).list_context_status()

    assert items[0]["local_port"] == get_unique_port("acme-prod", 20000, 5000)
