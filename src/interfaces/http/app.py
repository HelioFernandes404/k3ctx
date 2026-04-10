"""HTTP interface backed by shared application use cases."""

from __future__ import annotations

import argparse
import os
from json import JSONDecodeError
from pathlib import Path
from typing import Any, cast

import uvicorn
from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import JSONResponse, Response
from starlette.routing import Route

from src.application.use_cases.connect import connect_cluster as connect_cluster_use_case
from src.application.use_cases.contexts import (
    set_current_context as set_current_context_use_case,
)
from src.application.use_cases.inventory import (
    find_target_by_context_name,
    list_cluster_targets,
)
from src.application.use_cases.status import list_context_status
from src.application.use_cases.tunnels import kill_tunnel as kill_tunnel_use_case
from src.bootstrap import ServiceContainer, build_service_container
from src.config import load_effective_config
from src.domain.models import EffectiveConfig
from src.interfaces.serialization import (
    cluster_targets_payload,
    context_switch_payload,
    context_not_found_payload,
    effective_config_payload,
    operation_error_payload,
    to_jsonable,
    tunnel_kill_failure_payload,
    tunnel_kill_success_payload,
)

DEFAULT_HTTP_HOST = "127.0.0.1"
DEFAULT_HTTP_PORT = 8080


def _default_project_dir() -> Path:
    return Path(__file__).resolve().parents[3]


def _bool_value(payload: dict[str, Any], key: str, default: bool) -> tuple[bool | None, Response | None]:
    value = payload.get(key, default)
    if isinstance(value, bool):
        return value, None
    return None, JSONResponse(
        operation_error_payload("invalid_request", f"Field '{key}' must be a boolean"),
        status_code=400,
    )


async def _json_payload(request: Request) -> tuple[dict[str, Any] | None, Response | None]:
    try:
        payload = await request.json()
    except JSONDecodeError:
        return None, JSONResponse(
            operation_error_payload("invalid_request", "Request body must be valid JSON"),
            status_code=400,
        )

    if not isinstance(payload, dict):
        return None, JSONResponse(
            operation_error_payload("invalid_request", "Request body must be a JSON object"),
            status_code=400,
        )

    return cast(dict[str, Any], payload), None


def build_http_app(
    *,
    project_dir: Path | None = None,
    config_path: str | Path | None = None,
    services: ServiceContainer | None = None,
) -> Starlette:
    resolved_project_dir = project_dir or _default_project_dir()

    def current_config() -> EffectiveConfig:
        return load_effective_config(resolved_project_dir, config_path)

    def current_services() -> ServiceContainer:
        return services or build_service_container()

    async def get_config(request: Request) -> Response:
        del request
        return JSONResponse(effective_config_payload(current_config()))

    async def get_clusters(request: Request) -> Response:
        del request
        config = current_config()
        runtime = current_services()
        targets = list_cluster_targets(config.inventory_path, runtime.catalog)
        return JSONResponse(cluster_targets_payload(targets))

    async def get_status(request: Request) -> Response:
        del request
        runtime = current_services()
        items = list_context_status(runtime.status_reader)
        return JSONResponse(to_jsonable(items))

    async def post_connect(request: Request) -> Response:
        payload, error_response = await _json_payload(request)
        if error_response is not None:
            return error_response
        assert payload is not None

        context_name = payload.get("context_name")
        if not isinstance(context_name, str) or not context_name:
            return JSONResponse(
                operation_error_payload("invalid_request", "Field 'context_name' is required"),
                status_code=400,
            )

        allow_manual_network, error_response = _bool_value(
            payload,
            "allow_manual_network",
            True,
        )
        if error_response is not None:
            return error_response
        assert allow_manual_network is not None

        config = current_config()
        runtime = current_services()
        target = find_target_by_context_name(
            context_name,
            config.inventory_path,
            runtime.catalog,
        )
        if target is None:
            return JSONResponse(context_not_found_payload(context_name), status_code=404)

        result = connect_cluster_use_case(
            target=target,
            config=config,
            connector=runtime.connector,
            allow_manual_network=allow_manual_network,
        )
        return JSONResponse(result.to_public_dict())

    async def post_current_context(request: Request) -> Response:
        payload, error_response = await _json_payload(request)
        if error_response is not None:
            return error_response
        assert payload is not None

        context_name = payload.get("context_name")
        if not isinstance(context_name, str) or not context_name:
            return JSONResponse(
                operation_error_payload("invalid_request", "Field 'context_name' is required"),
                status_code=400,
            )

        require_confirmation, error_response = _bool_value(
            payload,
            "require_confirmation",
            True,
        )
        if error_response is not None:
            return error_response
        confirmed, error_response = _bool_value(payload, "confirmed", False)
        if error_response is not None:
            return error_response
        assert require_confirmation is not None
        assert confirmed is not None

        runtime = current_services()
        error = set_current_context_use_case(
            context_name,
            switcher=runtime.switcher,
            require_confirmation=require_confirmation,
            confirmed=confirmed,
        )
        return JSONResponse(context_switch_payload(context_name, error))

    async def post_kill_tunnel(request: Request) -> Response:
        context_name = request.path_params["context"]
        runtime = current_services()
        try:
            kill_tunnel_use_case(context_name, runtime.tunnel_manager)
        except Exception:
            return JSONResponse(tunnel_kill_failure_payload(context_name))

        return JSONResponse(tunnel_kill_success_payload(context_name))

    return Starlette(
        debug=False,
        routes=[
            Route("/config", get_config, methods=["GET"]),
            Route("/clusters", get_clusters, methods=["GET"]),
            Route("/status", get_status, methods=["GET"]),
            Route("/connect", post_connect, methods=["POST"]),
            Route("/contexts/current", post_current_context, methods=["POST"]),
            Route("/tunnels/{context:str}/kill", post_kill_tunnel, methods=["POST"]),
        ],
    )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="k3s-context-tunnel-manager-http")
    parser.add_argument("--host", default=os.getenv("HTTP_HOST", DEFAULT_HTTP_HOST))
    parser.add_argument(
        "--port",
        type=int,
        default=int(os.getenv("HTTP_PORT", str(DEFAULT_HTTP_PORT))),
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    uvicorn.run(
        build_http_app(),
        host=args.host,
        port=args.port,
        log_level="info",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
