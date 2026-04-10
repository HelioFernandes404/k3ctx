"""Kubectl-backed context switching adapter."""

from __future__ import annotations

import subprocess

from src.domain.models import OperationError
from src.logging_config import get_logger

logger = get_logger()


def _build_public_context_error() -> OperationError:
    return OperationError(
        code="kubectl_context_failed",
        message="Failed to switch kubectl context",
    )


class KubectlContextSwitcher:
    def switch_context(self, context_name: str) -> OperationError | None:
        logger.info(
            "Switching kubectl context via adapter",
            extra={
                "event": "infrastructure.contexts.switch.started",
                "context_name": context_name,
            },
        )
        try:
            result = subprocess.run(
                ["kubectl", "config", "use-context", context_name],
                capture_output=True,
                text=True,
                timeout=10,
            )
        except (FileNotFoundError, subprocess.TimeoutExpired):
            logger.error(
                "Kubectl context switch failed",
                extra={
                    "event": "infrastructure.contexts.switch.failed",
                    "context_name": context_name,
                },
            )
            return _build_public_context_error()

        if result.returncode != 0:
            logger.error(
                "Kubectl returned non-zero exit code during context switch",
                extra={
                    "event": "infrastructure.contexts.switch.failed",
                    "context_name": context_name,
                    "return_code": result.returncode,
                },
            )
            return _build_public_context_error()

        logger.info(
            "Kubectl context switched via adapter",
            extra={
                "event": "infrastructure.contexts.switch.finished",
                "context_name": context_name,
            },
        )
        return None
