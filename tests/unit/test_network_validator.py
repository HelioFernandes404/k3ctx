"""Unit tests for network validator error handling."""

from __future__ import annotations

import socket
import subprocess
from pathlib import Path
from unittest.mock import MagicMock

import pytest
from pytest_mock import MockerFixture

from src.network_validator import (
    check_sshuttle_active,
    get_network_metadata,
    validate_context_network,
    validate_context_network_details,
    validate_network_access,
)


def test_corrupted_network_metadata_returns_safe_failure(tmp_path: Path) -> None:
    state_dir = tmp_path / "state"
    state_dir.mkdir()
    (state_dir / "acme-prod.network").write_text("network_type: [broken")

    metadata = get_network_metadata("acme-prod", state_dir)
    ok, warning = validate_context_network("acme-prod", state_dir)
    details = validate_context_network_details("acme-prod", state_dir)

    assert isinstance(metadata, dict)
    assert metadata["corrupted"] is True
    assert ok is False
    assert warning is not None
    assert "could not be read safely" in warning
    assert details["ok"] is False
    assert details["warning"] == warning
    assert details["network_metadata"]["corrupted"] is True


# --- get_network_metadata ---


def test_get_network_metadata_returns_none_when_no_file(tmp_path: Path) -> None:
    result = get_network_metadata("acme-prod", tmp_path)

    assert result is None


def test_get_network_metadata_returns_empty_dict_for_empty_yaml_file(
    tmp_path: Path,
) -> None:
    (tmp_path / "acme-prod.network").write_text("")

    result = get_network_metadata("acme-prod", tmp_path)

    assert result == {}


def test_get_network_metadata_returns_content_for_valid_yaml(tmp_path: Path) -> None:
    (tmp_path / "acme-prod.network").write_text(
        "network_type: sshuttle\nnetwork_range: 10.0.0.0/24\n"
    )

    result = get_network_metadata("acme-prod", tmp_path)

    assert result == {"network_type": "sshuttle", "network_range": "10.0.0.0/24"}


def test_get_network_metadata_returns_corrupted_for_non_dict_yaml(
    tmp_path: Path,
) -> None:
    (tmp_path / "acme-prod.network").write_text("- item1\n- item2\n")

    result = get_network_metadata("acme-prod", tmp_path)

    assert result is not None
    assert result.get("corrupted") is True


# --- validate_context_network ---


def test_validate_context_network_returns_ok_when_no_file(tmp_path: Path) -> None:
    ok, warning = validate_context_network("acme-prod", tmp_path)

    assert ok is True
    assert warning is None


def test_validate_context_network_returns_ok_for_empty_metadata(tmp_path: Path) -> None:
    (tmp_path / "acme-prod.network").write_text("")

    ok, warning = validate_context_network("acme-prod", tmp_path)

    assert ok is True
    assert warning is None


def test_validate_context_network_returns_failure_when_needs_vpn(
    tmp_path: Path,
) -> None:
    (tmp_path / "acme-prod.network").write_text("needs_vpn: true\n")

    ok, warning = validate_context_network("acme-prod", tmp_path)

    assert ok is False
    assert warning == "This cluster requires VPN connection"


def test_validate_context_network_returns_failure_when_sshuttle_not_running(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    (tmp_path / "acme-prod.network").write_text(
        "network_type: sshuttle\nnetwork_range: 192.168.90.0/24\n"
    )
    mocker.patch("src.network_validator.check_sshuttle_active", return_value=False)

    ok, warning = validate_context_network("acme-prod", tmp_path)

    assert ok is False
    assert warning is not None
    assert "192.168.90.0/24" in warning


def test_validate_context_network_returns_ok_when_sshuttle_active(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    (tmp_path / "acme-prod.network").write_text(
        "network_type: sshuttle\nnetwork_range: 192.168.90.0/24\n"
    )
    mocker.patch("src.network_validator.check_sshuttle_active", return_value=True)

    ok, warning = validate_context_network("acme-prod", tmp_path)

    assert ok is True
    assert warning is None


def test_validate_context_network_returns_failure_when_sshuttle_network_range_missing(
    tmp_path: Path,
) -> None:
    (tmp_path / "acme-prod.network").write_text("network_type: sshuttle\n")

    ok, warning = validate_context_network("acme-prod", tmp_path)

    assert ok is False
    assert warning is not None


def test_validate_context_network_includes_sshuttle_command_hint_in_warning(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    (tmp_path / "acme-prod.network").write_text(
        "network_type: sshuttle\n"
        "network_range: 192.168.90.0/24\n"
        "sshuttle_command: sshuttle -v -r helio@bastion 192.168.90.0/24\n"
    )
    mocker.patch("src.network_validator.check_sshuttle_active", return_value=False)

    ok, warning = validate_context_network("acme-prod", tmp_path)

    assert ok is False
    assert warning is not None
    assert "sshuttle -v -r helio@bastion 192.168.90.0/24" in warning


# --- check_sshuttle_active ---


def test_check_sshuttle_active_returns_true_when_specific_range_found(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.network_validator.subprocess.run",
        return_value=subprocess.CompletedProcess([], 0, "12345\n", ""),
    )

    assert check_sshuttle_active("192.168.90.0/24") is True


def test_check_sshuttle_active_falls_back_to_generic_sshuttle_process(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.network_validator.subprocess.run",
        side_effect=[
            subprocess.CompletedProcess([], 1, "", ""),
            subprocess.CompletedProcess([], 0, "99999\n", ""),
        ],
    )

    assert check_sshuttle_active("192.168.90.0/24") is True


def test_check_sshuttle_active_returns_false_when_no_sshuttle_found(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.network_validator.subprocess.run",
        return_value=subprocess.CompletedProcess([], 1, "", ""),
    )

    assert check_sshuttle_active("192.168.90.0/24") is False


def test_check_sshuttle_active_returns_false_on_subprocess_timeout(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.network_validator.subprocess.run",
        side_effect=subprocess.TimeoutExpired(["pgrep"], 2),
    )

    assert check_sshuttle_active("192.168.90.0/24") is False


def test_check_sshuttle_active_returns_false_when_pgrep_not_installed(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.network_validator.subprocess.run",
        side_effect=FileNotFoundError("pgrep not found"),
    )

    assert check_sshuttle_active("192.168.90.0/24") is False


# --- validate_network_access ---


def test_validate_network_access_returns_true_on_successful_connection(
    mocker: MockerFixture,
) -> None:
    mock_sock = MagicMock()
    mock_sock.connect_ex.return_value = 0
    mocker.patch("src.network_validator.socket.socket", return_value=mock_sock)

    assert validate_network_access("10.0.0.10") is True
    mock_sock.close.assert_called_once()


def test_validate_network_access_returns_false_on_connection_refused(
    mocker: MockerFixture,
) -> None:
    mock_sock = MagicMock()
    mock_sock.connect_ex.return_value = 111
    mocker.patch("src.network_validator.socket.socket", return_value=mock_sock)

    assert validate_network_access("10.0.0.10") is False


def test_validate_network_access_returns_false_on_socket_exception(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.network_validator.socket.socket",
        side_effect=socket.error("network unreachable"),
    )

    assert validate_network_access("10.0.0.10") is False


# --- validate_context_network_details ---


def test_validate_context_network_details_returns_ok_when_no_file(
    tmp_path: Path,
) -> None:
    result = validate_context_network_details("acme-prod", tmp_path)

    assert result["context_name"] == "acme-prod"
    assert result["ok"] is True
    assert result["warning"] is None
    assert result["network_metadata"] is None


def test_validate_context_network_details_returns_failure_for_corrupted_file(
    tmp_path: Path,
) -> None:
    (tmp_path / "acme-prod.network").write_text("network_type: [broken")

    result = validate_context_network_details("acme-prod", tmp_path)

    assert result["ok"] is False
    assert result["network_metadata"] is not None
    assert result["network_metadata"]["corrupted"] is True


def test_validate_context_network_details_returns_ok_for_empty_metadata(
    tmp_path: Path,
) -> None:
    (tmp_path / "acme-prod.network").write_text("")

    result = validate_context_network_details("acme-prod", tmp_path)

    assert result["ok"] is True
    assert result["warning"] is None
