"""FastMCP interface backed by the shared application layer."""

from __future__ import annotations

import os
from pathlib import Path
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from fastmcp import FastMCP
else:
    try:
        from fastmcp import FastMCP
    except ImportError:  # pragma: no cover - compatibility fallback
        from mcp.server.fastmcp import FastMCP

from src.application.use_cases.connect import (
    connect_cluster as connect_cluster_use_case,
    connect_multiple as connect_multiple_use_case,
)
from src.application.use_cases.contexts import set_current_context as set_current_context_use_case
from src.application.use_cases.inventory import (
    deduplicate_context_names,
    find_target_by_context_name,
    list_cluster_targets,
    select_targets_by_context_name,
)
from src.application.use_cases.status import (
    list_context_status,
    validate_context_network as validate_context_network_use_case,
)
from src.application.use_cases.tunnels import kill_tunnel as kill_tunnel_use_case
from src.bootstrap import ServiceContainer, build_service_container
from src.config import load_effective_config
from src.domain.models import EffectiveConfig
from src.interfaces.serialization import (
    cluster_targets_payload,
    context_switch_payload,
    context_not_found_payload,
    effective_config_payload,
    json_resource,
    tunnel_kill_failure_payload,
    tunnel_kill_success_payload,
)


def _default_project_dir() -> Path:
    return Path(__file__).resolve().parents[3]


PROJECT_NAME = "k3s-context-tunnel-manager"

def build_mcp_server(
    *,
    project_dir: Path | None = None,
    config_path: str | Path | None = None,
) -> FastMCP:
    resolved_project_dir = project_dir or _default_project_dir()
    server = FastMCP(PROJECT_NAME)

    def current_config() -> EffectiveConfig:
        return load_effective_config(resolved_project_dir, config_path)

    def services() -> ServiceContainer:
        return build_service_container()

    @server.tool()
    def connect_cluster(
        context_name: str,
        allow_manual_network: bool = True,
    ) -> dict[str, Any]:
        config = current_config()
        runtime = services()
        target = find_target_by_context_name(
            context_name,
            config.inventory_path,
            runtime.catalog,
        )
        if target is None:
            return context_not_found_payload(context_name)

        return connect_cluster_use_case(
            target=target,
            config=config,
            connector=runtime.connector,
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
        runtime = services()
        requested_contexts = deduplicate_context_names(context_names)
        targets, missing_contexts = select_targets_by_context_name(
            requested_contexts,
            config.inventory_path,
            runtime.catalog,
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

        results = connect_multiple_use_case(
            targets=targets,
            config=config,
            connector=runtime.connector,
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
        runtime = services()
        error = set_current_context_use_case(
            context_name,
            switcher=runtime.switcher,
            require_confirmation=require_confirmation,
            confirmed=confirmed,
        )
        return context_switch_payload(context_name, error)

    @server.tool()
    def kill_tunnel(context_name: str) -> dict[str, Any]:
        runtime = services()
        try:
            kill_tunnel_use_case(context_name, runtime.tunnel_manager)
        except Exception:
            return tunnel_kill_failure_payload(context_name)

        return tunnel_kill_success_payload(context_name)

    @server.tool()
    def validate_context_network(context_name: str) -> dict[str, Any]:
        runtime = services()
        return validate_context_network_use_case(context_name, runtime.status_reader)

    @server.resource(
        "inventory://clusters",
        mime_type="application/json",
        annotations={"readOnlyHint": True},
    )
    def inventory_clusters() -> str:
        config = current_config()
        runtime = services()
        targets = list_cluster_targets(config.inventory_path, runtime.catalog)
        return json_resource(cluster_targets_payload(targets))

    @server.resource(
        "status://contexts",
        mime_type="application/json",
        annotations={"readOnlyHint": True},
    )
    def status_contexts() -> str:
        runtime = services()
        return json_resource(list_context_status(runtime.status_reader))

    @server.resource(
        "config://effective",
        mime_type="application/json",
        annotations={"readOnlyHint": True},
    )
    def effective_config() -> str:
        return json_resource(effective_config_payload(current_config()))

    return server


mcp = build_mcp_server()


def main_stdio() -> None:
    os.environ.setdefault("FASTMCP_LOG_ENABLED", "false")
    os.environ.setdefault("FASTMCP_SHOW_SERVER_BANNER", "false")
    mcp.run(show_banner=False)


if __name__ == "__main__":
    main_stdio()


__all__ = ["build_mcp_server", "main_stdio", "mcp"]
