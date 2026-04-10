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
