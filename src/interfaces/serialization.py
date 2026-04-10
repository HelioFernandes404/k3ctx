"""Shared serializers for machine-readable interfaces."""

from __future__ import annotations

import json
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import Any

from src.domain.discovery import (
    ClientPage,
    ClientSummary,
    HostPage,
    HostQuery,
    HostRecord,
    HostResolutionResult,
    PageInfo,
)
from src.domain.models import ClusterTarget, EffectiveConfig, OperationError


def to_jsonable(value: Any) -> Any:
    if isinstance(value, Path):
        return str(value)
    if isinstance(value, Mapping):
        return {str(key): to_jsonable(item) for key, item in value.items()}
    if isinstance(value, Sequence) and not isinstance(value, (str, bytes, bytearray)):
        return [to_jsonable(item) for item in value]
    if hasattr(value, "__dataclass_fields__"):
        return {
            field_name: to_jsonable(getattr(value, field_name))
            for field_name in value.__dataclass_fields__
        }
    return value


def json_resource(payload: Any) -> str:
    return json.dumps(to_jsonable(payload), sort_keys=True)


def effective_config_payload(config: EffectiveConfig) -> dict[str, Any]:
    return {
        "inventory_path": str(config.inventory_path),
        "ssh_config_path": config.ssh_config_path,
        "ssh_key_path": config.ssh_key_path,
        "remote_k3s_config_path": config.remote_k3s_config_path,
        "k3s_api_port": config.k3s_api_port,
        "port_range_start": config.port_range_start,
        "port_range_size": config.port_range_size,
    }


def cluster_target_payload(target: ClusterTarget) -> dict[str, Any]:
    return {
        "company": target.company,
        "host_alias": target.host_alias,
        "group": target.group,
        "context_name": target.context_name,
        "host_config": to_jsonable(target.host_config),
        "group_vars": to_jsonable(target.group_vars),
    }


def cluster_targets_payload(targets: Sequence[ClusterTarget]) -> list[dict[str, Any]]:
    return [cluster_target_payload(target) for target in targets]


def page_info_payload(page: PageInfo) -> dict[str, Any]:
    return {
        "limit": page.limit,
        "returned": page.returned,
        "total": page.total,
        "has_more": page.has_more,
        "next_cursor": page.next_cursor,
    }


def client_summary_payload(summary: ClientSummary) -> dict[str, Any]:
    return {
        "client": summary.client,
        "host_count": summary.host_count,
    }


def client_page_payload(page: ClientPage) -> dict[str, Any]:
    return {
        "ok": True,
        "items": [client_summary_payload(item) for item in page.items],
        "pagination": page_info_payload(page.page),
    }


def host_query_payload(query: HostQuery) -> dict[str, Any]:
    return {
        "client": query.client,
        "host_name": query.host_name,
        "systemframe_id": query.systemframe_id,
        "addr_ip": query.addr_ip,
        "context_name": query.context_name,
        "query": query.query,
        "exact": query.exact,
    }


def host_record_payload(record: HostRecord) -> dict[str, Any]:
    return {
        "client": record.client,
        "host_name": record.host_name,
        "systemframe_id": record.systemframe_id,
        "addr_ip": record.addr_ip,
        "context_name": record.context_name,
        "group": record.group,
    }


def host_page_payload(page: HostPage) -> dict[str, Any]:
    return {
        "ok": True,
        "query": host_query_payload(page.query),
        "items": [host_record_payload(item) for item in page.items],
        "pagination": page_info_payload(page.page),
    }


def _resolution_error_code(status: str) -> str:
    return "ambiguous_target" if status == "ambiguous" else "no_match"


def _suggested_commands(
    result: HostResolutionResult,
    *,
    cli_name: str,
) -> list[str]:
    if result.status != "ambiguous" or not result.matches:
        return []

    first = result.matches[0]
    commands = [f"{cli_name} hosts {first.client} --host {first.host_name}"]
    if first.addr_ip is not None:
        commands.append(f"{cli_name} connect --ip {first.addr_ip}")
    if first.systemframe_id is not None:
        commands.append(f"{cli_name} connect --id {first.systemframe_id}")
    return commands


def host_resolution_payload(
    result: HostResolutionResult,
    *,
    cli_name: str,
) -> dict[str, Any]:
    return {
        "ok": False,
        "error": {
            "code": _resolution_error_code(result.status),
            "message": result.hint or "Host resolution failed",
        },
        "query": host_query_payload(result.query),
        "matches": [host_record_payload(item) for item in result.matches],
        "pagination": page_info_payload(result.page),
        "hint": result.hint,
        "suggested_commands": _suggested_commands(result, cli_name=cli_name),
    }


def context_not_found_payload(context_name: str) -> dict[str, Any]:
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


def operation_error_payload(code: str, message: str) -> dict[str, Any]:
    return {
        "error": {
            "code": code,
            "message": message,
            "retryable": False,
        }
    }


def context_switch_payload(
    context_name: str,
    error: OperationError | None,
) -> dict[str, Any]:
    return {
        "success": error is None,
        "context_name": context_name,
        "error": None if error is None else error.to_public_dict(),
    }


def tunnel_kill_success_payload(context_name: str) -> dict[str, Any]:
    return {
        "success": True,
        "context_name": context_name,
        "error": None,
    }


def tunnel_kill_failure_payload(context_name: str) -> dict[str, Any]:
    return {
        "success": False,
        "context_name": context_name,
        "error": {
            "code": "kill_tunnel_failed",
            "message": "Failed to stop tunnel",
            "retryable": False,
        },
    }
