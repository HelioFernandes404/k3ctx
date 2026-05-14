"""Tests for interface-agnostic inventory use cases."""

from __future__ import annotations

from pathlib import Path

from src.application.use_cases.inventory import (
    deduplicate_context_names,
    find_target_by_context_name,
    list_cluster_targets,
    refresh_inventory_if_possible,
    select_targets_by_context_name,
)
from src.domain.models import ClusterTarget


class StubCatalog:
    def __init__(self, targets: list[ClusterTarget]) -> None:
        self.targets = targets
        self.calls: list[Path] = []

    def list_targets(self, inventory_path: Path) -> list[ClusterTarget]:
        self.calls.append(inventory_path)
        return list(self.targets)


class StubRefresher:
    def __init__(self, result: tuple[bool, str]) -> None:
        self.result = result
        self.calls: list[Path] = []

    def refresh(self, inventory_path: Path) -> tuple[bool, str]:
        self.calls.append(inventory_path)
        return self.result


def test_deduplicate_context_names_preserves_order() -> None:
    assert deduplicate_context_names(["beta-dev", "acme-prod", "beta-dev"]) == [
        "beta-dev",
        "acme-prod",
    ]


def test_deduplicate_context_names_returns_empty_list_for_empty_input() -> None:
    assert deduplicate_context_names([]) == []


def test_deduplicate_context_names_returns_single_item_unchanged() -> None:
    assert deduplicate_context_names(["acme-prod"]) == ["acme-prod"]


def test_find_target_by_context_name_returns_matching_target(tmp_path: Path) -> None:
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "203.0.113.10"},
        group_vars={},
    )
    catalog = StubCatalog([target])

    result = find_target_by_context_name("acme-prod", tmp_path, catalog)

    assert result == target
    assert catalog.calls == [tmp_path]


def test_find_target_by_context_name_returns_none_when_not_found(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog([])

    result = find_target_by_context_name("acme-prod", tmp_path, catalog)

    assert result is None


def test_find_target_by_context_name_returns_none_when_context_not_in_catalog(
    tmp_path: Path,
) -> None:
    target = ClusterTarget(
        company="acme",
        host_alias="dev",
        group="k3s_cluster",
        host_config={"ansible_host": "203.0.113.10"},
        group_vars={},
    )
    catalog = StubCatalog([target])

    result = find_target_by_context_name("acme-prod", tmp_path, catalog)

    assert result is None


def test_list_cluster_targets_delegates_to_catalog(tmp_path: Path) -> None:
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "203.0.113.10"},
        group_vars={},
    )
    catalog = StubCatalog([target])

    result = list_cluster_targets(tmp_path, catalog)

    assert result == [target]
    assert catalog.calls == [tmp_path]


def test_list_cluster_targets_returns_empty_for_empty_catalog(tmp_path: Path) -> None:
    catalog = StubCatalog([])

    result = list_cluster_targets(tmp_path, catalog)

    assert result == []


def test_select_targets_by_context_name_returns_missing_contexts(tmp_path: Path) -> None:
    first = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "203.0.113.10"},
        group_vars={},
    )
    second = ClusterTarget(
        company="beta",
        host_alias="staging",
        group="k3s_cluster",
        host_config={"ansible_host": "203.0.113.11"},
        group_vars={},
    )
    catalog = StubCatalog([first, second])

    selected, missing = select_targets_by_context_name(
        ["beta-staging", "missing", "acme-prod"],
        tmp_path,
        catalog,
    )

    assert selected == [second, first]
    assert missing == ["missing"]


def test_select_targets_by_context_name_returns_all_missing_when_catalog_empty(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog([])

    selected, missing = select_targets_by_context_name(
        ["acme-prod", "beta-staging"],
        tmp_path,
        catalog,
    )

    assert selected == []
    assert missing == ["acme-prod", "beta-staging"]


def test_refresh_inventory_if_possible_returns_none_when_path_does_not_exist(
    tmp_path: Path,
) -> None:
    missing_path = tmp_path / "does-not-exist"
    refresher = StubRefresher((True, "ok"))

    result = refresh_inventory_if_possible(missing_path, refresher)

    assert result is None
    assert refresher.calls == []


def test_refresh_inventory_if_possible_calls_refresher_when_path_exists(
    tmp_path: Path,
) -> None:
    inventory_dir = tmp_path / "inventory"
    inventory_dir.mkdir()
    refresher = StubRefresher((True, "Updated successfully"))

    result = refresh_inventory_if_possible(inventory_dir, refresher)

    assert result == (True, "Updated successfully")
    assert refresher.calls == [inventory_dir]


def test_refresh_inventory_if_possible_propagates_failure_from_refresher(
    tmp_path: Path,
) -> None:
    inventory_dir = tmp_path / "inventory"
    inventory_dir.mkdir()
    refresher = StubRefresher((False, "git pull failed"))

    result = refresh_inventory_if_possible(inventory_dir, refresher)

    assert result == (False, "git pull failed")
