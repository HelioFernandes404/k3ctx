"""Output ports used by the application layer."""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import TYPE_CHECKING, Protocol

from src.domain.models import (
    ClusterTarget,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)

if TYPE_CHECKING:
    from src.domain.argocd import ArgocdConfig


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
    argocd_local_port: int | None = None


@dataclass(frozen=True)
class ArgocdLoginResult:
    success: bool
    local_port: int | None
    skipped: bool = False
    message: str = ""


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


class ArgocdConnector(Protocol):
    def setup(
        self,
        context_name: str,
        argocd_config: "ArgocdConfig",
        *,
        hostname: str,
        username: str,
        keyfile: str | None,
        port: int,
        proxycmd: str | None,
        internal_ip: str,
    ) -> ArgocdLoginResult: ...
