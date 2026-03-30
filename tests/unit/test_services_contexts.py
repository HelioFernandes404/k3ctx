"""Unit tests for context mutation helpers."""

import subprocess

from pytest_mock import MockerFixture

from src.models import OperationError
from src.services.contexts import set_current_context


def test_set_current_context_requires_explicit_confirmation(
    mocker: MockerFixture,
) -> None:
    mock_run = mocker.patch("src.services.contexts.subprocess.run")

    error = set_current_context(
        "acme-prod",
        require_confirmation=True,
        confirmed=False,
    )

    assert isinstance(error, OperationError)
    assert error.code == "confirmation_required"
    mock_run.assert_not_called()


def test_set_current_context_returns_error_when_kubectl_fails(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.services.contexts.subprocess.run",
        return_value=subprocess.CompletedProcess(
            args=["kubectl"],
            returncode=1,
            stdout="",
            stderr="context missing token=abc123",
        ),
    )

    error = set_current_context(
        "acme-prod",
        require_confirmation=False,
        confirmed=False,
    )

    assert isinstance(error, OperationError)
    assert error.code == "kubectl_context_failed"
    assert error.detail is None


def test_set_current_context_returns_safe_public_error_when_kubectl_is_missing(
    mocker: MockerFixture,
) -> None:
    mocker.patch(
        "src.services.contexts.subprocess.run",
        side_effect=FileNotFoundError("kubectl missing private_key=/tmp/key"),
    )

    error = set_current_context(
        "acme-prod",
        require_confirmation=False,
        confirmed=False,
    )

    assert isinstance(error, OperationError)
    assert error.code == "kubectl_context_failed"
    assert error.detail is None


def test_set_current_context_returns_none_on_success(
    mocker: MockerFixture,
) -> None:
    mock_run = mocker.patch(
        "src.services.contexts.subprocess.run",
        return_value=subprocess.CompletedProcess(
            args=["kubectl"],
            returncode=0,
            stdout="Switched to context\n",
            stderr="",
        ),
    )

    error = set_current_context(
        "acme-prod",
        require_confirmation=True,
        confirmed=True,
    )

    assert error is None
    mock_run.assert_called_once_with(
        ["kubectl", "config", "use-context", "acme-prod"],
        capture_output=True,
        text=True,
        timeout=10,
    )
