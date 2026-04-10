"""Inventory adapters backed by local YAML files and git."""

from __future__ import annotations

from pathlib import Path

from src.domain.models import ClusterTarget
from src.inventory import extract_hosts_from_inventory, load_inventories, update_inventory_repo
from src.logging_config import get_logger

logger = get_logger()


class YamlInventoryCatalog:
    def list_targets(self, inventory_path: Path) -> list[ClusterTarget]:
        logger.info(
            "Listing cluster targets from inventory adapter",
            extra={
                "event": "infrastructure.inventory.list.started",
                "inventory_path": inventory_path,
            },
        )
        targets: list[ClusterTarget] = []

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

        logger.info(
            "Listed cluster targets from inventory adapter",
            extra={
                "event": "infrastructure.inventory.list.finished",
                "inventory_path": inventory_path,
                "target_count": len(targets),
            },
        )
        return targets


class GitInventoryRefresher:
    def refresh(self, inventory_path: Path) -> tuple[bool, str]:
        return update_inventory_repo(inventory_path)
