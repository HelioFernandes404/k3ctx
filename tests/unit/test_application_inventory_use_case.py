"""Tests for interface-agnostic inventory use cases."""

from __future__ import annotations

from pathlib import Path

from src.application.use_cases.inventory import (
    deduplicate_context_names,
    find_target_by_context_name,
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


def test_deduplicate_context_names_preserves_order() -> None:
    assert deduplicate_context_names(["beta-dev", "acme-prod", "beta-dev"]) == [
        "beta-dev",
        "acme-prod",
    ]


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
