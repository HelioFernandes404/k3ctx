"""Unit tests for the LocalArgocdConnector infrastructure adapter."""

from __future__ import annotations

from unittest.mock import MagicMock

import pytest
from pytest_mock import MockerFixture

from src.domain.argocd import ArgocdConfig
from src.infrastructure.adapters.argocd_connector import (
    LocalArgocdConnector,
    _fetch_argocd_password,
)


def _ssh_kwargs() -> dict:
    return {
        "hostname": "203.0.113.10",
        "username": "helio",
        "keyfile": "/home/helio/.ssh/id_ed25519",
        "port": 22,
        "proxycmd": None,
        "internal_ip": "10.0.0.1",
    }


class TestLocalArgocdConnectorSkipped:
    def test_skips_when_argocd_disabled(self, mocker: MockerFixture) -> None:
        connector = LocalArgocdConnector()
        result = connector.setup(
            "acme-prod",
            ArgocdConfig.disabled(),
            **_ssh_kwargs(),
        )
        assert result.skipped is True
        assert result.local_port is None

    def test_skips_when_node_port_missing(self, mocker: MockerFixture) -> None:
        connector = LocalArgocdConnector()
        result = connector.setup(
            "acme-prod",
            ArgocdConfig(enabled=True, node_port=None),
            **_ssh_kwargs(),
        )
        assert result.skipped is True
        assert result.local_port is None


class TestLocalArgocdConnectorTunnel:
    def test_opens_tunnel_when_not_running(self, mocker: MockerFixture) -> None:
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.is_tunnel_running",
            return_value=False,
        )
        create_tunnel = mocker.patch(
            "src.infrastructure.adapters.argocd_connector.create_tunnel",
            return_value=12345,
        )
        save_pid = mocker.patch(
            "src.infrastructure.adapters.argocd_connector.save_tunnel_pid",
        )
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.shutil.which",
            return_value=None,
        )

        connector = LocalArgocdConnector()
        result = connector.setup(
            "acme-prod",
            ArgocdConfig(enabled=True, node_port=30080),
            **_ssh_kwargs(),
        )

        create_tunnel.assert_called_once()
        save_pid.assert_called_once()
        assert result.local_port is not None

    def test_reuses_existing_tunnel(self, mocker: MockerFixture) -> None:
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.is_tunnel_running",
            return_value=True,
        )
        create_tunnel = mocker.patch(
            "src.infrastructure.adapters.argocd_connector.create_tunnel",
        )
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.shutil.which",
            return_value=None,
        )

        connector = LocalArgocdConnector()
        connector.setup(
            "acme-prod",
            ArgocdConfig(enabled=True, node_port=30080),
            **_ssh_kwargs(),
        )

        create_tunnel.assert_not_called()


class TestLocalArgocdConnectorLogin:
    def _patch_tunnel(self, mocker: MockerFixture) -> None:
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.is_tunnel_running",
            return_value=True,
        )

    def test_returns_failure_when_argocd_cli_absent(self, mocker: MockerFixture) -> None:
        self._patch_tunnel(mocker)
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.shutil.which",
            return_value=None,
        )

        connector = LocalArgocdConnector()
        result = connector.setup(
            "acme-prod",
            ArgocdConfig(enabled=True, node_port=30080),
            **_ssh_kwargs(),
        )

        assert result.success is False
        assert result.skipped is False
        assert result.local_port is not None
        assert "argocd CLI not found" in result.message

    def test_returns_failure_when_password_unavailable(self, mocker: MockerFixture) -> None:
        self._patch_tunnel(mocker)
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.shutil.which",
            return_value="/usr/bin/argocd",
        )
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector._fetch_argocd_password",
            return_value=None,
        )

        connector = LocalArgocdConnector()
        result = connector.setup(
            "acme-prod",
            ArgocdConfig(enabled=True, node_port=30080),
            **_ssh_kwargs(),
        )

        assert result.success is False
        assert result.local_port is not None
        assert "argocd login --insecure" in result.message

    def test_runs_argocd_login_when_password_available(self, mocker: MockerFixture) -> None:
        self._patch_tunnel(mocker)
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.shutil.which",
            return_value="/usr/bin/argocd",
        )
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector._fetch_argocd_password",
            return_value="s3cr3t",
        )
        run_mock = mocker.patch(
            "src.infrastructure.adapters.argocd_connector.subprocess.run",
        )

        connector = LocalArgocdConnector()
        result = connector.setup(
            "acme-prod",
            ArgocdConfig(enabled=True, node_port=30080),
            **_ssh_kwargs(),
        )

        assert result.success is True
        assert result.local_port is not None
        cmd = run_mock.call_args[0][0]
        assert cmd[0] == "argocd"
        assert "admin" in cmd
        assert "s3cr3t" in cmd
        assert "--insecure" in cmd

    def test_uses_plaintext_flag_when_configured(self, mocker: MockerFixture) -> None:
        self._patch_tunnel(mocker)
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.shutil.which",
            return_value="/usr/bin/argocd",
        )
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector._fetch_argocd_password",
            return_value="s3cr3t",
        )
        run_mock = mocker.patch(
            "src.infrastructure.adapters.argocd_connector.subprocess.run",
        )

        connector = LocalArgocdConnector()
        connector.setup(
            "acme-prod",
            ArgocdConfig(enabled=True, node_port=30080, plaintext=True),
            **_ssh_kwargs(),
        )

        cmd = run_mock.call_args[0][0]
        assert "--plaintext" in cmd
        assert "--insecure" not in cmd

    def test_returns_failure_on_login_subprocess_error(self, mocker: MockerFixture) -> None:
        import subprocess as sp

        self._patch_tunnel(mocker)
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.shutil.which",
            return_value="/usr/bin/argocd",
        )
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector._fetch_argocd_password",
            return_value="s3cr3t",
        )
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.subprocess.run",
            side_effect=sp.CalledProcessError(1, "argocd"),
        )

        connector = LocalArgocdConnector()
        result = connector.setup(
            "acme-prod",
            ArgocdConfig(enabled=True, node_port=30080),
            **_ssh_kwargs(),
        )

        assert result.success is False
        assert "argocd login failed" in result.message


class TestFetchArgocdPassword:
    def test_returns_none_on_nonzero_returncode(self, mocker: MockerFixture) -> None:
        proc = MagicMock()
        proc.returncode = 1
        proc.stdout = ""
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.subprocess.run",
            return_value=proc,
        )

        result = _fetch_argocd_password("acme-prod", "argocd")
        assert result is None

    def test_returns_none_on_empty_stdout(self, mocker: MockerFixture) -> None:
        proc = MagicMock()
        proc.returncode = 0
        proc.stdout = ""
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.subprocess.run",
            return_value=proc,
        )

        result = _fetch_argocd_password("acme-prod", "argocd")
        assert result is None

    def test_decodes_base64_password(self, mocker: MockerFixture) -> None:
        import base64

        raw = "supersecret"
        encoded = base64.b64encode(raw.encode()).decode()
        proc = MagicMock()
        proc.returncode = 0
        proc.stdout = encoded
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.subprocess.run",
            return_value=proc,
        )

        result = _fetch_argocd_password("acme-prod", "argocd")
        assert result == raw

    def test_returns_none_on_exception(self, mocker: MockerFixture) -> None:
        mocker.patch(
            "src.infrastructure.adapters.argocd_connector.subprocess.run",
            side_effect=Exception("unexpected"),
        )

        result = _fetch_argocd_password("acme-prod", "argocd")
        assert result is None
