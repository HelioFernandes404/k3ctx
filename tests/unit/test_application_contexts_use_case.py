"""Tests for interface-agnostic context switch use cases."""

from __future__ import annotations

from src.application.use_cases.contexts import set_current_context
from src.domain.models import OperationError


class StubSwitcher:
    def __init__(self, result: OperationError | None) -> None:
        self.result = result
        self.calls: list[str] = []

    def switch_context(self, context_name: str) -> OperationError | None:
        self.calls.append(context_name)
        return self.result


def test_set_current_context_requires_explicit_confirmation() -> None:
    switcher = StubSwitcher(None)

    error = set_current_context(
        "acme-prod",
        switcher,
        require_confirmation=True,
        confirmed=False,
    )

    assert isinstance(error, OperationError)
    assert error.code == "confirmation_required"
    assert switcher.calls == []


def test_set_current_context_delegates_to_switcher_when_confirmed() -> None:
    switcher = StubSwitcher(None)

    error = set_current_context(
        "acme-prod",
        switcher,
        require_confirmation=True,
        confirmed=True,
    )

    assert error is None
    assert switcher.calls == ["acme-prod"]


def test_set_current_context_delegates_when_confirmation_not_required() -> None:
    switcher = StubSwitcher(None)

    error = set_current_context(
        "acme-prod",
        switcher,
        require_confirmation=False,
        confirmed=False,
    )

    assert error is None
    assert switcher.calls == ["acme-prod"]


def test_set_current_context_propagates_switcher_error() -> None:
    kubectl_error = OperationError(
        code="kubectl_context_failed",
        message="Failed to switch kubectl context",
    )
    switcher = StubSwitcher(kubectl_error)

    error = set_current_context(
        "acme-prod",
        switcher,
        require_confirmation=False,
        confirmed=False,
    )

    assert error is kubectl_error
    assert switcher.calls == ["acme-prod"]


def test_set_current_context_propagates_switcher_error_even_when_confirmed() -> None:
    kubectl_error = OperationError(
        code="kubectl_context_failed",
        message="Failed to switch kubectl context",
    )
    switcher = StubSwitcher(kubectl_error)

    error = set_current_context(
        "acme-prod",
        switcher,
        require_confirmation=True,
        confirmed=True,
    )

    assert error is kubectl_error


def test_set_current_context_does_not_call_switcher_without_confirmation() -> None:
    switcher = StubSwitcher(None)

    set_current_context(
        "acme-prod",
        switcher,
        require_confirmation=True,
        confirmed=False,
    )

    assert switcher.calls == []
