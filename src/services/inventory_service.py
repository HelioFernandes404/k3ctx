"""Read-only inventory service helpers."""

from __future__ import annotations

from pathlib import Path

from src.application.use_cases.inventory import (
    list_cluster_targets as list_cluster_targets_use_case,
)
from src.bootstrap import build_service_container
from src.logging_config import get_logger
from src.models import ClusterTarget

logger = get_logger()


def list_cluster_targets(inventory_path: Path) -> list[ClusterTarget]:
    logger.info(
        "Listing cluster targets from inventory",
        extra={
            "event": "inventory.list.started",
            "inventory_path": inventory_path,
        },
    )
    services = build_service_container()

    try:
        targets = list_cluster_targets_use_case(inventory_path, services.catalog)
    except Exception as exc:
        logger.error(
            "Failed to list cluster targets from inventory",
            extra={
                "event": "inventory.list.failed",
                "inventory_path": inventory_path,
                "error_type": type(exc).__name__,
            },
        )
        raise

    logger.info(
        "Listed cluster targets from inventory",
        extra={
            "event": "inventory.list.finished",
            "inventory_path": inventory_path,
            "target_count": len(targets),
        },
    )
    return targets
