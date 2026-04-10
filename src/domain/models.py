"""Core typed models for cluster connection flows."""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from types import MappingProxyType
from typing import Any, Mapping, Optional


def _freeze_value(value: Any) -> Any:
    if isinstance(value, dict):
        return MappingProxyType({key: _freeze_value(inner) for key, inner in value.items()})
    if isinstance(value, list):
        return tuple(_freeze_value(item) for item in value)
    if isinstance(value, tuple):
        return tuple(_freeze_value(item) for item in value)
    if isinstance(value, set):
        return frozenset(_freeze_value(item) for item in value)
    return value


@dataclass(frozen=True)
class EffectiveConfig:
    inventory_path: Path
    ssh_config_path: str
    ssh_key_path: str
    remote_k3s_config_path: str
    k3s_api_port: int
    port_range_start: int
    port_range_size: int


@dataclass(frozen=True)
class NetworkRequirement:
    type: Optional[str]
    network_range: Optional[str]
    needs_vpn: bool = False

    @classmethod
    def none(cls) -> "NetworkRequirement":
        return cls(type=None, network_range=None, needs_vpn=False)


@dataclass(frozen=True)
class ClusterTarget:
    company: str
    host_alias: str
    group: str
    host_config: Mapping[str, Any] = field(default_factory=dict)
    group_vars: Mapping[str, Any] = field(default_factory=dict)

    def __post_init__(self) -> None:
        frozen_host_config = {
            key: _freeze_value(value) for key, value in self.host_config.items()
        }
        frozen_group_vars = {
            key: _freeze_value(value) for key, value in self.group_vars.items()
        }
        object.__setattr__(self, "host_config", MappingProxyType(frozen_host_config))
        object.__setattr__(self, "group_vars", MappingProxyType(frozen_group_vars))

    @property
    def context_name(self) -> str:
        return f"{self.company}-{self.host_alias}"


@dataclass(frozen=True)
class OperationError:
    code: str
    message: str
    detail: Optional[str] = None
    retryable: bool = False

    def to_public_dict(self) -> dict[str, Any]:
        return {
            "code": self.code,
            "message": self.message,
            "retryable": self.retryable,
        }


@dataclass(frozen=True)
class ConnectResult:
    success: bool
    context_name: str
    local_port: Optional[int]
    internal_ip: Optional[str]
    tunnel_pid: Optional[int]
    used_cache: bool
    network_requirement: NetworkRequirement
    error: Optional[OperationError] = None

    def __post_init__(self) -> None:
        if self.success and self.error is not None:
            raise ValueError("success=True requires error=None")
        if not self.success and self.error is None:
            raise ValueError("success=False requires error to be set")

    def to_public_dict(self) -> dict[str, Any]:
        return {
            "success": self.success,
            "context_name": self.context_name,
            "local_port": self.local_port,
            "internal_ip": self.internal_ip,
            "tunnel_pid": self.tunnel_pid,
            "used_cache": self.used_cache,
            "network_requirement": {
                "type": self.network_requirement.type,
                "network_range": self.network_requirement.network_range,
                "needs_vpn": self.network_requirement.needs_vpn,
            },
            "error": None if self.error is None else self.error.to_public_dict(),
        }
