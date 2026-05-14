"""Default wiring for application ports and infrastructure adapters."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from src.application.ports import (
    ClusterConnector,
    ContextSwitcher,
    InventoryCatalog,
    InventoryRefresher,
    StatusReader,
    TunnelManager,
)
from src.infrastructure.adapters.argocd_connector import LocalArgocdConnector
from src.infrastructure.adapters.cluster_connector import LocalClusterConnector
from src.infrastructure.adapters.context_switcher import KubectlContextSwitcher
from src.infrastructure.adapters.inventory_catalog import (
    GitInventoryRefresher,
    YamlInventoryCatalog,
)
from src.infrastructure.adapters.status_reader import LocalStatusReader
from src.infrastructure.adapters.tunnel_manager import LocalTunnelManager


@dataclass(frozen=True)
class ServiceContainer:
    catalog: InventoryCatalog
    refresher: InventoryRefresher
    connector: ClusterConnector
    switcher: ContextSwitcher
    status_reader: StatusReader
    tunnel_manager: TunnelManager


def build_service_container(state_dir: Path | None = None) -> ServiceContainer:
    return ServiceContainer(
        catalog=YamlInventoryCatalog(),
        refresher=GitInventoryRefresher(),
        connector=LocalClusterConnector(argocd_connector=LocalArgocdConnector()),
        switcher=KubectlContextSwitcher(),
        status_reader=LocalStatusReader() if state_dir is None else LocalStatusReader(state_dir),
        tunnel_manager=LocalTunnelManager(),
    )
