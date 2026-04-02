"""FastMCP server layer for the k9s setup core services."""

from __future__ import annotations

import json
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from fastmcp import FastMCP
else:
    try:
        from fastmcp import FastMCP
    except ImportError:  # pragma: no cover - compatibility fallback
        from mcp.server.fastmcp import FastMCP

from src.config import load_effective_config
from src.models import ClusterTarget, EffectiveConfig
from src.services.connect import (
    connect_cluster as connect_cluster_service,
    connect_multiple as connect_multiple_service,
)
from src.services.contexts import set_current_context as set_current_context_service
from src.services.inventory_service import list_cluster_targets
from src.services.status import (
    list_context_status,
    validate_context_network as validate_context_network_service,
)
from src.tunnel import kill_tunnel as kill_tunnel_service


def _default_project_dir() -> Path:
    return Path(__file__).resolve().parent.parent


def _to_jsonable(value: Any) -> Any:
    if isinstance(value, Path):
        return str(value)
    if isinstance(value, Mapping):
        return {str(key): _to_jsonable(item) for key, item in value.items()}
    if isinstance(value, Sequence) and not isinstance(value, (str, bytes, bytearray)):
        return [_to_jsonable(item) for item in value]
    if hasattr(value, "__dataclass_fields__"):
        return {
            field_name: _to_jsonable(getattr(value, field_name))
            for field_name in value.__dataclass_fields__
        }
    return value


def _json_resource(payload: Any) -> str:
    return json.dumps(_to_jsonable(payload), sort_keys=True)


def _effective_config_payload(config: EffectiveConfig) -> dict[str, Any]:
    return {
        "inventory_path": str(config.inventory_path),
        "ssh_config_path": config.ssh_config_path,
        "ssh_key_path": config.ssh_key_path,
        "remote_k3s_config_path": config.remote_k3s_config_path,
        "k3s_api_port": config.k3s_api_port,
        "port_range_start": config.port_range_start,
        "port_range_size": config.port_range_size,
    }


def _target_payload(target: ClusterTarget) -> dict[str, Any]:
    return {
        "company": target.company,
        "host_alias": target.host_alias,
        "group": target.group,
        "context_name": target.context_name,
        "host_config": _to_jsonable(target.host_config),
        "group_vars": _to_jsonable(target.group_vars),
    }


def _find_target(context_name: str, inventory_path: Path) -> ClusterTarget | None:
    for target in list_cluster_targets(inventory_path):
        if target.context_name == context_name:
            return target
    return None


def _select_targets(
    requested_contexts: list[str],
    inventory_path: Path,
) -> tuple[list[ClusterTarget], list[str]]:
    targets = list_cluster_targets(inventory_path)
    by_context = {target.context_name: target for target in targets}
    selected: list[ClusterTarget] = []
    missing: list[str] = []
    for context_name in requested_contexts:
        target = by_context.get(context_name)
        if target is None:
            missing.append(context_name)
            continue
        selected.append(target)
    return selected, missing


def _deduplicate_context_names(context_names: list[str]) -> list[str]:
    unique_contexts: list[str] = []
    seen: set[str] = set()
    for context_name in context_names:
        if context_name in seen:
            continue
        seen.add(context_name)
        unique_contexts.append(context_name)
    return unique_contexts


def build_mcp_server(
    *,
    project_dir: Path | None = None,
    config_path: str | Path | None = None,
) -> FastMCP:
    resolved_project_dir = project_dir or _default_project_dir()
    server = FastMCP("k9s-setup")

    def current_config() -> EffectiveConfig:
        return load_effective_config(resolved_project_dir, config_path)

    @server.tool()
    def connect_cluster(
        context_name: str,
        allow_manual_network: bool = True,
    ) -> dict[str, Any]:
        config = current_config()
        target = _find_target(context_name, config.inventory_path)
        if target is None:
            return {
                "success": False,
                "context_name": context_name,
                "local_port": None,
                "internal_ip": None,
                "tunnel_pid": None,
                "used_cache": False,
                "network_requirement": {
                    "type": None,
                    "network_range": None,
                    "needs_vpn": False,
                },
                "error": {
                    "code": "context_not_found",
                    "message": "Cluster context not found in inventory",
                    "retryable": False,
                },
            }

        return connect_cluster_service(
            target=target,
            config=config,
            allow_manual_network=allow_manual_network,
        ).to_public_dict()

    @server.tool()
    def connect_multiple(
        context_names: list[str] | None = None,
        allow_manual_network: bool = True,
    ) -> dict[str, Any]:
        if not context_names:
            return {
                "success": False,
                "results": [],
                "missing_contexts": [],
                "error": {
                    "code": "context_names_required",
                    "message": "At least one context_name must be provided",
                    "retryable": False,
                },
            }

        config = current_config()
        requested_contexts = _deduplicate_context_names(context_names)
        targets, missing_contexts = _select_targets(
            requested_contexts,
            config.inventory_path,
        )
        if not targets:
            return {
                "success": False,
                "results": [],
                "missing_contexts": missing_contexts,
                "error": {
                    "code": "no_valid_contexts",
                    "message": "No valid cluster contexts were found in inventory",
                    "retryable": False,
                },
            }

        results = connect_multiple_service(
            targets=targets,
            config=config,
            allow_manual_network=allow_manual_network,
        )
        return {
            "success": True,
            "results": [result.to_public_dict() for result in results],
            "missing_contexts": missing_contexts,
            "error": None,
        }

    @server.tool()
    def set_current_context(
        context_name: str,
        require_confirmation: bool = True,
        confirmed: bool = False,
    ) -> dict[str, Any]:
        error = set_current_context_service(
            context_name,
            require_confirmation=require_confirmation,
            confirmed=confirmed,
        )
        return {
            "success": error is None,
            "context_name": context_name,
            "error": None if error is None else error.to_public_dict(),
        }

    @server.tool()
    def kill_tunnel(context_name: str) -> dict[str, Any]:
        try:
            kill_tunnel_service(context_name)
        except Exception:
            return {
                "success": False,
                "context_name": context_name,
                "error": {
                    "code": "kill_tunnel_failed",
                    "message": "Failed to stop tunnel",
                    "retryable": False,
                },
            }

        return {
            "success": True,
            "context_name": context_name,
            "error": None,
        }

    @server.tool()
    def validate_context_network(context_name: str) -> dict[str, Any]:
        return validate_context_network_service(context_name)

    @server.resource(
        "inventory://clusters",
        mime_type="application/json",
        annotations={"readOnlyHint": True},
    )
    def inventory_clusters() -> str:
        config = current_config()
        targets = list_cluster_targets(config.inventory_path)
        return _json_resource([_target_payload(target) for target in targets])

    @server.resource(
        "status://contexts",
        mime_type="application/json",
        annotations={"readOnlyHint": True},
    )
    def status_contexts() -> str:
        return _json_resource(list_context_status())

    @server.resource(
        "config://effective",
        mime_type="application/json",
        annotations={"readOnlyHint": True},
    )
    def effective_config() -> str:
        return _json_resource(_effective_config_payload(current_config()))

    return server


mcp = build_mcp_server()


if __name__ == "__main__":
    mcp.run(show_banner=False)


__all__ = ["build_mcp_server", "mcp"]
