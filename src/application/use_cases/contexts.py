"""Context-related application use cases."""

from __future__ import annotations

from src.application.ports import ContextSwitcher
from src.domain.models import OperationError


def set_current_context(
    context_name: str,
    switcher: ContextSwitcher,
    *,
    require_confirmation: bool,
    confirmed: bool,
) -> OperationError | None:
    if require_confirmation and not confirmed:
        return OperationError(
            code="confirmation_required",
            message="Context switch requires explicit confirmation",
        )

    return switcher.switch_context(context_name)
