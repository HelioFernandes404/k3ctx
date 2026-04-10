"""Mutating helpers for Kubernetes context selection."""

from __future__ import annotations

from src.application.use_cases.contexts import (
    set_current_context as set_current_context_use_case,
)
from src.bootstrap import build_service_container
from src.logging_config import get_logger
from src.models import OperationError

logger = get_logger()


def set_current_context(
    context_name: str,
    *,
    require_confirmation: bool,
    confirmed: bool,
) -> OperationError | None:
    logger.info(
        "Switching kubectl context",
        extra={
            "event": "contexts.switch.started",
            "context_name": context_name,
            "require_confirmation": require_confirmation,
            "confirmed": confirmed,
        },
    )
    services = build_service_container()
    error = set_current_context_use_case(
        context_name,
        switcher=services.switcher,
        require_confirmation=require_confirmation,
        confirmed=confirmed,
    )

    if error is None:
        logger.info(
            "Kubectl context switched",
            extra={
                "event": "contexts.switch.finished",
                "context_name": context_name,
            },
        )
    elif error.code == "confirmation_required":
        logger.warning(
            "Context switch requires confirmation",
            extra={
                "event": "contexts.switch.confirmation_required",
                "context_name": context_name,
            },
        )
    else:
        logger.error(
            "Kubectl context switch failed",
            extra={
                "event": "contexts.switch.failed",
                "context_name": context_name,
                "error_code": error.code,
            },
        )

    return error
