"""Local status reader adapter."""

from __future__ import annotations

from pathlib import Path

from src.logging_config import get_logger
from src.network_validator import get_network_metadata, validate_context_network_details
from src.status_runtime import (
    get_current_context,
    get_tunnel_pid,
    get_tunnel_port,
    list_all_context_names,
    load_status_config,
)
from src.tunnel import TUNNEL_STATE_DIR, is_tunnel_running

logger = get_logger()


class LocalStatusReader:
    def __init__(self, state_dir: Path = TUNNEL_STATE_DIR) -> None:
        self.state_dir = state_dir

    def list_context_status(self) -> list[dict[str, object]]:
        logger.info(
            "Listing context status via adapter",
            extra={
                "event": "infrastructure.status.list.started",
                "state_dir": self.state_dir,
            },
        )
        current_context = get_current_context()
        config = load_status_config()
        items: list[dict[str, object]] = []

        for context_name in list_all_context_names(self.state_dir):
            items.append(
                {
                    "name": context_name,
                    "is_current": context_name == current_context,
                    "tunnel_running": is_tunnel_running(context_name, self.state_dir),
                    "tunnel_pid": get_tunnel_pid(context_name, self.state_dir),
                    "local_port": get_tunnel_port(context_name, config),
                    "network_metadata": get_network_metadata(context_name, self.state_dir),
                }
            )

        logger.info(
            "Listed context status via adapter",
            extra={
                "event": "infrastructure.status.list.finished",
                "context_count": len(items),
            },
        )
        return items

    def validate_context_network(self, context_name: str) -> dict[str, object]:
        logger.info(
            "Validating context network via adapter",
            extra={
                "event": "infrastructure.status.validate_network.started",
                "context_name": context_name,
            },
        )
        return validate_context_network_details(context_name, self.state_dir)
