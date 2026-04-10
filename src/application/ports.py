"""Output ports used by the application layer."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

from src.domain.models import (
    ClusterTarget,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)


class ClusterConnectionError(RuntimeError):
    """Connector failure with a public-safe error contract."""

    def __init__(
        self,
        *,
        code: str,
        message: str,
        detail: str | None = None,
        retryable: bool = False,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.detail = detail
        self.retryable = retryable

    def to_operation_error(self) -> OperationError:
        return OperationError(
            code=self.code,
            message=self.message,
            detail=self.detail,
            retryable=self.retryable,
        )


@dataclass(frozen=True)
class ConnectionArtifacts:
    local_port: int
    internal_ip: str
    tunnel_pid: int | None
    used_cache: bool


class InventoryCatalog(Protocol):
    def list_targets(self, inventory_path: Path) -> list[ClusterTarget]: ...


class InventoryRefresher(Protocol):
    def refresh(self, inventory_path: Path) -> tuple[bool, str]: ...


class ClusterConnector(Protocol):
    def connect(
        self,
        target: ClusterTarget,
        config: EffectiveConfig,
        requirement: NetworkRequirement,
    ) -> ConnectionArtifacts: ...


class ContextSwitcher(Protocol):
    def switch_context(self, context_name: str) -> OperationError | None: ...


class StatusReader(Protocol):
    def list_context_status(self) -> list[dict[str, object]]: ...

    def validate_context_network(self, context_name: str) -> dict[str, object]: ...


class TunnelManager(Protocol):
    def kill_tunnel(self, context_name: str) -> None: ...
