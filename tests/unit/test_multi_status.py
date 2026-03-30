"""Unit tests for legacy multi-status output."""

from pathlib import Path

from _pytest.capture import CaptureFixture
from pytest_mock import MockerFixture

from src.models import EffectiveConfig
from src.multi_status import list_all_contexts, show_status
from src.tunnel import get_unique_port


def test_legacy_status_marks_corrupted_network_metadata_safely(
    tmp_path: Path,
    mocker: MockerFixture,
    capsys: CaptureFixture[str],
) -> None:
    state_dir = tmp_path / "state"
    state_dir.mkdir()
    (state_dir / "acme-prod.network").write_text("network_type: [broken")

    mocker.patch("src.multi_status.get_current_context", return_value=None)
    mocker.patch("src.multi_status.is_tunnel_running", return_value=False)

    contexts = list_all_contexts(state_dir)

    assert contexts[0]["name"] == "acme-prod"
    assert contexts[0]["network_validation"]["ok"] is False
    assert "could not be read safely" in contexts[0]["network_validation"]["warning"]

    show_status(state_dir)
    output = capsys.readouterr().out

    assert "acme-prod" in output
    assert "network metadata unreadable" in output.lower()


def test_legacy_status_uses_canonical_port_range(
    tmp_path: Path,
    mocker: MockerFixture,
) -> None:
    state_dir = tmp_path / "state"
    state_dir.mkdir()
    (state_dir / "acme-prod.pid").write_text("4242")

    mocker.patch("src.multi_status.get_current_context", return_value=None)
    mocker.patch("src.multi_status.is_tunnel_running", return_value=True)
    mocker.patch("src.multi_status.get_tunnel_pid", return_value=4242)
    mocker.patch("src.multi_status.get_network_metadata", return_value=None)
    mocker.patch(
        "src.multi_status.load_status_config",
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

    contexts = list_all_contexts(state_dir)

    assert contexts[0]["local_port"] == get_unique_port("acme-prod", 20000, 5000)


def test_legacy_status_shows_operational_network_warnings_when_tunnel_down(
    tmp_path: Path,
    mocker: MockerFixture,
    capsys: CaptureFixture[str],
) -> None:
    state_dir = tmp_path / "state"
    state_dir.mkdir()

    mocker.patch(
        "src.multi_status.list_all_contexts",
        return_value=[
            {
                "name": "acme-vpn",
                "tunnel_running": False,
                "tunnel_pid": None,
                "local_port": 20101,
                "network_metadata": {"needs_vpn": True},
                "network_validation": {
                    "ok": False,
                    "warning": "This cluster requires VPN connection",
                },
            },
            {
                "name": "acme-sshuttle",
                "tunnel_running": False,
                "tunnel_pid": None,
                "local_port": 20102,
                "network_metadata": {
                    "network_type": "sshuttle",
                    "network_range": "10.0.0.0/24",
                },
                "network_validation": {
                    "ok": False,
                    "warning": "This cluster requires sshuttle for 10.0.0.0/24",
                },
            },
        ],
    )
    mocker.patch("src.multi_status.get_current_context", return_value=None)

    show_status(state_dir)
    output = capsys.readouterr().out.lower()

    assert "acme-vpn" in output
    assert "requires vpn" in output
    assert "acme-sshuttle" in output
    assert "requires sshuttle" in output
