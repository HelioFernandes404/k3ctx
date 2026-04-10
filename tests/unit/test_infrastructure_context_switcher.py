"""Tests for the kubectl-backed context switch adapter."""

from __future__ import annotations

import subprocess

from pytest_mock import MockerFixture

from src.infrastructure.adapters.context_switcher import KubectlContextSwitcher


def test_switch_context_returns_error_when_kubectl_fails(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.infrastructure.adapters.context_switcher.subprocess.run",
        return_value=subprocess.CompletedProcess(
            args=["kubectl"],
            returncode=1,
            stdout="",
            stderr="context missing token=abc123",
        ),
    )

    error = KubectlContextSwitcher().switch_context("acme-prod")

    assert error is not None
    assert error.code == "kubectl_context_failed"
    assert error.detail is None


def test_switch_context_returns_safe_public_error_when_kubectl_is_missing(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.infrastructure.adapters.context_switcher.subprocess.run",
        side_effect=FileNotFoundError("kubectl missing private_key=/tmp/key"),
    )

    error = KubectlContextSwitcher().switch_context("acme-prod")

    assert error is not None
    assert error.code == "kubectl_context_failed"
    assert error.detail is None


def test_switch_context_returns_none_on_success(
    mocker: MockerFixture,
) -> None:
    mock_run = mocker.patch(
        "src.infrastructure.adapters.context_switcher.subprocess.run",
        return_value=subprocess.CompletedProcess(
            args=["kubectl"],
            returncode=0,
            stdout="Switched to context\n",
            stderr="",
        ),
    )

    error = KubectlContextSwitcher().switch_context("acme-prod")

    assert error is None
    mock_run.assert_called_once_with(
        ["kubectl", "config", "use-context", "acme-prod"],
        capture_output=True,
        text=True,
        timeout=10,
    )
