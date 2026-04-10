"""Unit tests for legacy multi-status compatibility wrappers."""

from __future__ import annotations

from pathlib import Path

from _pytest.capture import CaptureFixture
from pytest_mock import MockerFixture

from src.multi_status import list_all_contexts, show_status


def test_legacy_status_list_all_contexts_combines_status_and_network_validation(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    list_status = mocker.patch(
        "src.multi_status.list_context_status",
        return_value=[
            {
                "name": "beta-dev",
                "is_current": False,
                "tunnel_running": False,
                "tunnel_pid": 9999,
                "local_port": 16444,
                "network_metadata": None,
            },
            {
                "name": "acme-prod",
                "is_current": True,
                "tunnel_running": True,
                "tunnel_pid": 4242,
                "local_port": 16443,
                "network_metadata": {"network_type": "sshuttle"},
            },
        ],
    )
    validate_network = mocker.patch(
        "src.multi_status.validate_context_network",
        side_effect=[
            {"context_name": "beta-dev", "ok": True},
            {"context_name": "acme-prod", "ok": False, "warning": "requires sshuttle"},
        ],
    )

    contexts = list_all_contexts(tmp_path)

    assert contexts == [
        {
            "name": "acme-prod",
            "is_current": True,
            "tunnel_running": True,
            "tunnel_pid": 4242,
            "local_port": 16443,
            "network_metadata": {"network_type": "sshuttle"},
            "network_validation": {
                "context_name": "acme-prod",
                "ok": False,
                "warning": "requires sshuttle",
            },
        },
        {
            "name": "beta-dev",
            "is_current": False,
            "tunnel_running": False,
            "tunnel_pid": None,
            "local_port": 16444,
            "network_metadata": None,
            "network_validation": {
                "context_name": "beta-dev",
                "ok": True,
            },
        },
    ]
    list_status.assert_called_once_with(tmp_path)
    validate_network.assert_any_call("beta-dev", tmp_path)
    validate_network.assert_any_call("acme-prod", tmp_path)


def test_legacy_status_show_status_reuses_shared_cli_presenter(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch(
        "src.multi_status.list_all_contexts",
        return_value=[
            {
                "name": "acme-prod",
                "is_current": True,
                "tunnel_running": True,
                "tunnel_pid": 4242,
                "local_port": 16443,
                "network_metadata": {"network_type": "sshuttle"},
                "network_validation": {
                    "context_name": "acme-prod",
                    "ok": False,
                    "warning": "requires sshuttle",
                },
            }
        ],
    )
    print_status = mocker.patch("src.multi_status.print_status")

    show_status(tmp_path)

    print_status.assert_called_once_with(
        [
            {
                "name": "acme-prod",
                "is_current": True,
                "tunnel_running": True,
                "tunnel_pid": 4242,
                "local_port": 16443,
                "network_metadata": {"network_type": "sshuttle"},
            }
        ],
        {
            "acme-prod": {
                "context_name": "acme-prod",
                "ok": False,
                "warning": "requires sshuttle",
            }
        },
    )


def test_legacy_status_show_status_keeps_warning_output_contract(
    mocker: MockerFixture,
    capsys: CaptureFixture[str],
) -> None:
    mocker.patch(
        "src.multi_status.list_all_contexts",
        return_value=[
            {
                "name": "acme-vpn",
                "is_current": False,
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
                "is_current": False,
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

    show_status()
    output = capsys.readouterr().out.lower()

    assert "acme-vpn" in output
    assert "requires vpn" in output
    assert "acme-sshuttle" in output
    assert "requires sshuttle" in output
