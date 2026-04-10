"""Unit tests for discovery-focused inventory use cases."""

from __future__ import annotations

from pathlib import Path

from src.application.use_cases.discovery import load_host_records
from src.domain.discovery import HostRecord
from src.domain.models import ClusterTarget


class StubCatalog:
    def __init__(self, targets: list[ClusterTarget]) -> None:
        self.targets = targets
        self.calls: list[Path] = []

    def list_targets(self, inventory_path: Path) -> list[ClusterTarget]:
        self.calls.append(inventory_path)
        return list(self.targets)


def build_target(
    *,
    company: str = "acme",
    host_alias: str = "prod",
    addr_ip: str = "10.0.0.10",
    systemframe_id: str | None = None,
    group_systemframe_id: str | None = None,
) -> ClusterTarget:
    host_config: dict[str, object] = {"ansible_host": addr_ip}
    if systemframe_id is not None:
        host_config["systemframe_id"] = systemframe_id

    group_vars: dict[str, object] = {}
    if group_systemframe_id is not None:
        group_vars["systemframe_id"] = group_systemframe_id

    return ClusterTarget(
        company=company,
        host_alias=host_alias,
        group="k3s_cluster",
        host_config=host_config,
        group_vars=group_vars,
    )


def test_load_host_records_projects_public_fields_from_cluster_targets(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [build_target(systemframe_id="sf-1042")]
    )

    records = load_host_records(tmp_path, catalog)

    assert records == [
        HostRecord(
            client="acme",
            host_name="prod",
            systemframe_id="sf-1042",
            addr_ip="10.0.0.10",
            context_name="acme-prod",
            group="k3s_cluster",
        )
    ]
    assert catalog.calls == [tmp_path]


def test_load_host_records_falls_back_to_group_vars_for_systemframe_id(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [build_target(systemframe_id=None, group_systemframe_id="sf-group-7")]
    )

    records = load_host_records(tmp_path, catalog)

    assert records[0].systemframe_id == "sf-group-7"
