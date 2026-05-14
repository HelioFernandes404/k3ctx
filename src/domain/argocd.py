"""ArgoCD integration domain model."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Mapping, Optional


@dataclass(frozen=True)
class ArgocdConfig:
    enabled: bool
    namespace: str = "argocd"
    node_port: Optional[int] = None
    plaintext: bool = False

    @classmethod
    def disabled(cls) -> "ArgocdConfig":
        return cls(enabled=False)

    @classmethod
    def from_host_config(
        cls,
        host_config: Mapping[str, Any],
        group_vars: Optional[Mapping[str, Any]] = None,
    ) -> "ArgocdConfig":
        def _get(key: str, default: Any = None) -> Any:
            val = host_config.get(key)
            if val is None and group_vars is not None:
                val = group_vars.get(key)
            return val if val is not None else default

        enabled = bool(_get("argocd_enabled", False))
        if not enabled:
            return cls.disabled()

        namespace = str(_get("argocd_namespace", "argocd"))
        node_port_raw = _get("argocd_node_port")
        node_port = int(node_port_raw) if node_port_raw is not None else None
        plaintext = bool(_get("argocd_plaintext", False))

        return cls(enabled=True, namespace=namespace, node_port=node_port, plaintext=plaintext)
