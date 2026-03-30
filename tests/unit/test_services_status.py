"""Unit tests for the status service."""

from pathlib import Path

from pytest_mock import MockerFixture

from src.models import EffectiveConfig
from src.services.status import list_context_status, validate_context_network
from src.tunnel import get_unique_port


def test_list_context_status_reads_tunnel_and_network_metadata(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch("src.services.status.get_current_context", return_value="acme-prod")
    mocker.patch(
        "src.services.status.list_all_context_names",
        return_value=["acme-prod", "acme-dev"],
    )
    mocker.patch(
        "src.services.status.is_tunnel_running",
        side_effect=[True, False],
    )
    mocker.patch(
        "src.services.status.get_tunnel_pid",
        side_effect=[4242, None],
    )
    mocker.patch(
        "src.services.status.get_tunnel_port",
        side_effect=[16443, 16444],
    )
    mocker.patch(
        "src.services.status.get_network_metadata",
        side_effect=[
            {"network_type": "sshuttle", "network_range": "10.0.0.0/24"},
            None,
        ],
    )

    items = list_context_status(tmp_path)

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
        "src.services.status.validate_context_network_details",
        return_value={
            "context_name": "acme-prod",
            "ok": False,
            "warning": "This cluster requires VPN connection",
            "network_metadata": {"needs_vpn": True},
        },
    )

    result = validate_context_network("acme-prod")

    assert result["context_name"] == "acme-prod"
    assert result["ok"] is False
    assert result["network_metadata"] == {"needs_vpn": True}


def test_list_context_status_uses_canonical_port_range(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch("src.services.status.get_current_context", return_value=None)
    mocker.patch(
        "src.services.status.list_all_context_names",
        return_value=["acme-prod"],
    )
    mocker.patch("src.services.status.is_tunnel_running", return_value=True)
    mocker.patch("src.services.status.get_tunnel_pid", return_value=4242)
    mocker.patch("src.services.status.get_network_metadata", return_value=None)
    mocker.patch(
        "src.services.status.load_status_config",
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

    items = list_context_status(tmp_path)

    assert items[0]["local_port"] == get_unique_port("acme-prod", 20000, 5000)
