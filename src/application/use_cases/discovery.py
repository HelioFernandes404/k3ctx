"""Discovery-focused application use cases."""

from __future__ import annotations

from pathlib import Path

from src.application.ports import InventoryCatalog
from src.application.use_cases.inventory import list_cluster_targets
from src.domain.discovery import HostRecord
from src.domain.models import ClusterTarget


def _optional_str(value: object) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    return text or None


def _systemframe_id_for(target: ClusterTarget) -> str | None:
    return _optional_str(
        target.host_config.get("systemframe_id")
        or target.group_vars.get("systemframe_id")
    )


def project_target_to_host_record(target: ClusterTarget) -> HostRecord:
    return HostRecord(
        client=target.company,
        host_name=target.host_alias,
        systemframe_id=_systemframe_id_for(target),
        addr_ip=_optional_str(target.host_config.get("ansible_host")),
        context_name=target.context_name,
        group=target.group,
    )


def load_host_records(
    inventory_path: Path,
    catalog: InventoryCatalog,
) -> list[HostRecord]:
    return [
        project_target_to_host_record(target)
        for target in list_cluster_targets(inventory_path, catalog)
    ]
