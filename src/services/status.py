"""Read-only status helpers for contexts and tunnels."""

from __future__ import annotations

from pathlib import Path

from src.logging_config import get_logger
from src.network_validator import (
    get_network_metadata,
    validate_context_network_details,
)
from src.status_runtime import (
    get_current_context,
    get_tunnel_pid,
    get_tunnel_port,
    list_all_context_names,
    load_status_config,
)
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
        current_context = get_current_context()
        config = load_status_config()
        items: list[dict[str, object]] = []

        for context_name in list_all_context_names(state_dir):
            items.append(
                {
                    "name": context_name,
                    "is_current": context_name == current_context,
                    "tunnel_running": is_tunnel_running(context_name, state_dir),
                    "tunnel_pid": get_tunnel_pid(context_name, state_dir),
                    "local_port": get_tunnel_port(context_name, config),
                    "network_metadata": get_network_metadata(context_name, state_dir),
                }
            )

        logger.info(
            "Listed context status",
            extra={
                "event": "status.list.finished",
                "state_dir": state_dir,
                "context_count": len(items),
                "current_context": current_context,
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
        result = validate_context_network_details(context_name, state_dir)
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
