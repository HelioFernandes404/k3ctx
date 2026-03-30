"""Mutating helpers for Kubernetes context selection."""

from __future__ import annotations

import subprocess

from src.logging_config import get_logger
from src.models import OperationError

logger = get_logger()


def _build_public_context_error() -> OperationError:
    return OperationError(
        code="kubectl_context_failed",
        message="Failed to switch kubectl context",
    )


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
    if require_confirmation and not confirmed:
        logger.warning(
            "Context switch requires confirmation",
            extra={
                "event": "contexts.switch.confirmation_required",
                "context_name": context_name,
            },
        )
        return OperationError(
            code="confirmation_required",
            message="Context switch requires explicit confirmation",
        )

    try:
        result = subprocess.run(
            ["kubectl", "config", "use-context", context_name],
            capture_output=True,
            text=True,
            timeout=10,
        )
    except (FileNotFoundError, subprocess.TimeoutExpired) as exc:
        logger.error(
            "Kubectl context switch failed",
            extra={
                "event": "contexts.switch.failed",
                "context_name": context_name,
                "error_type": type(exc).__name__,
            },
        )
        return _build_public_context_error()

    if result.returncode != 0:
        logger.error(
            "Kubectl returned non-zero exit code during context switch",
            extra={
                "event": "contexts.switch.failed",
                "context_name": context_name,
                "return_code": result.returncode,
                "stderr_present": bool(result.stderr.strip()),
                "stdout_present": bool(result.stdout.strip()),
            },
        )
        return _build_public_context_error()

    logger.info(
        "Kubectl context switched",
        extra={
            "event": "contexts.switch.finished",
            "context_name": context_name,
        },
    )
    return None
