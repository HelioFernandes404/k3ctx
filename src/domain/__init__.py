"""Core domain objects and pure policies."""

from src.domain.discovery import (
    ClientPage,
    ClientSummary,
    HostPage,
    HostQuery,
    HostRecord,
    HostResolutionResult,
    PageInfo,
)
from src.domain.models import (
    ClusterTarget,
    ConnectResult,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)

__all__ = [
    "ClientPage",
    "ClientSummary",
    "ClusterTarget",
    "ConnectResult",
    "EffectiveConfig",
    "HostPage",
    "HostQuery",
    "HostRecord",
    "HostResolutionResult",
    "NetworkRequirement",
    "OperationError",
    "PageInfo",
]
