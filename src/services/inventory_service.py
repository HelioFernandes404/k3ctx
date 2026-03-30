"""Read-only inventory service helpers."""

from __future__ import annotations

from pathlib import Path

from src.inventory import extract_hosts_from_inventory, load_inventories
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
    targets: list[ClusterTarget] = []

    try:
        for company, inv_data in sorted(load_inventories(inventory_path).items()):
            hosts = extract_hosts_from_inventory(inv_data)
            for host_alias, host_info in sorted(hosts.items()):
                targets.append(
                    ClusterTarget(
                        company=company,
                        host_alias=host_alias,
                        group=host_info["group"],
                        host_config=host_info["config"],
                        group_vars=host_info.get("group_vars", {}),
                    )
                )
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
