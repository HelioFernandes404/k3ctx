"""Read-only status helpers for contexts and tunnels."""

from __future__ import annotations

from pathlib import Path

from src.application.use_cases.status import (
    list_context_status as list_context_status_use_case,
)
from src.application.use_cases.status import (
    validate_context_network as validate_context_network_use_case,
)
from src.bootstrap import build_service_container
from src.logging_config import get_logger
from src.tunnel import TUNNEL_STATE_DIR, is_tunnel_running

logger = get_logger()


def list_context_status(state_dir: Path = TUNNEL_STATE_DIR) -> list[dict[str, object]]:
    logger.info(
        "Listing context status",
        extra={
            "event": "status.list.started",
            "state_dir": state_dir,
        },
    )
    try:
        services = build_service_container(state_dir)
        items = list_context_status_use_case(services.status_reader)

        logger.info(
            "Listed context status",
            extra={
                "event": "status.list.finished",
                "state_dir": state_dir,
                "context_count": len(items),
            },
        )
        return items
    except Exception as exc:
        logger.error(
            "Failed to list context status",
            extra={
                "event": "status.list.failed",
                "state_dir": state_dir,
                "error_type": type(exc).__name__,
            },
        )
        raise


def validate_context_network(
    context_name: str,
    state_dir: Path = TUNNEL_STATE_DIR,
) -> dict[str, object]:
    logger.info(
        "Validating context network",
        extra={
            "event": "status.validate_network.started",
            "context_name": context_name,
            "state_dir": state_dir,
        },
    )
    try:
        services = build_service_container(state_dir)
        result = validate_context_network_use_case(context_name, services.status_reader)
        logger.info(
            "Validated context network",
            extra={
                "event": "status.validate_network.finished",
                "context_name": context_name,
                "ok": result.get("ok"),
            },
        )
        return result
    except Exception as exc:
        logger.error(
            "Failed to validate context network",
            extra={
                "event": "status.validate_network.failed",
                "context_name": context_name,
                "error_type": type(exc).__name__,
            },
        )
        raise
