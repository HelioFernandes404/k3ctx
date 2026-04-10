"""Inventory-focused application use cases."""

from __future__ import annotations

from pathlib import Path

from src.application.ports import InventoryCatalog, InventoryRefresher
from src.domain.models import ClusterTarget


def list_cluster_targets(
    inventory_path: Path,
    catalog: InventoryCatalog,
) -> list[ClusterTarget]:
    return catalog.list_targets(inventory_path)


def find_target_by_context_name(
    context_name: str,
    inventory_path: Path,
    catalog: InventoryCatalog,
) -> ClusterTarget | None:
    for target in list_cluster_targets(inventory_path, catalog):
        if target.context_name == context_name:
            return target
    return None


def deduplicate_context_names(context_names: list[str]) -> list[str]:
    unique_contexts: list[str] = []
    seen: set[str] = set()
    for context_name in context_names:
        if context_name in seen:
            continue
        seen.add(context_name)
        unique_contexts.append(context_name)
    return unique_contexts


def select_targets_by_context_name(
    context_names: list[str],
    inventory_path: Path,
    catalog: InventoryCatalog,
) -> tuple[list[ClusterTarget], list[str]]:
    available_targets = list_cluster_targets(inventory_path, catalog)
    by_context = {target.context_name: target for target in available_targets}
    selected: list[ClusterTarget] = []
    missing: list[str] = []

    for context_name in context_names:
        target = by_context.get(context_name)
        if target is None:
            missing.append(context_name)
            continue
        selected.append(target)

    return selected, missing


def refresh_inventory_if_possible(
    inventory_path: Path,
    refresher: InventoryRefresher,
) -> tuple[bool, str] | None:
    if not inventory_path.exists():
        return None
    return refresher.refresh(inventory_path)
