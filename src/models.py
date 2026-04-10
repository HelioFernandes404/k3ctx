"""Compatibility wrapper for domain models."""

from src.domain.models import (
    ClusterTarget,
    ConnectResult,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)

__all__ = [
    "ClusterTarget",
    "ConnectResult",
    "EffectiveConfig",
    "NetworkRequirement",
    "OperationError",
]
