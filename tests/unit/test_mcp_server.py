"""Unit tests for the FastMCP server layer."""

from __future__ import annotations

import asyncio
import json
from pathlib import Path
from typing import Any, cast

from pytest_mock import MockerFixture

from src.models import ClusterTarget, ConnectResult, EffectiveConfig, NetworkRequirement
from src.mcp_server import build_mcp_server, mcp


def build_config(tmp_path: Path) -> EffectiveConfig:
    return EffectiveConfig(
        inventory_path=tmp_path / "inventory",
        ssh_config_path="~/.ssh/config",
        ssh_key_path="~/.ssh/id_ed25519",
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )


def test_build_mcp_server_registers_expected_tools_and_read_only_resources(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch(
        "src.mcp_server.load_effective_config",
        return_value=build_config(tmp_path),
    )
    mocker.patch("src.mcp_server.list_cluster_targets", return_value=[])
    mocker.patch("src.mcp_server.list_context_status", return_value=[])

    server = build_mcp_server(project_dir=tmp_path)

    tools = asyncio.run(server.list_tools())
    resources = asyncio.run(server.list_resources())

    assert {tool.name for tool in tools} == {
        "connect_cluster",
        "connect_multiple",
        "set_current_context",
        "kill_tunnel",
        "validate_context_network",
    }
    assert {str(resource.uri) for resource in resources} == {
        "inventory://clusters",
        "status://contexts",
        "config://effective",
    }
    assert all(
        resource.annotations is not None
        and cast(Any, resource.annotations).readOnlyHint is True
        for resource in resources
    )


def test_module_exports_default_mcp_server_instance() -> None:
    assert mcp is not None


def test_build_mcp_server_uses_public_project_name(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch(
        "src.mcp_server.load_effective_config",
        return_value=build_config(tmp_path),
    )
    mocker.patch("src.mcp_server.list_cluster_targets", return_value=[])
    mocker.patch("src.mcp_server.list_context_status", return_value=[])

    server = build_mcp_server(project_dir=tmp_path)

    assert getattr(server, "name", None) == "k3s-context-tunnel-manager"


def test_connect_cluster_tool_delegates_to_core(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )
    mocker.patch("src.mcp_server.load_effective_config", return_value=config)
    mocker.patch("src.mcp_server.list_cluster_targets", return_value=[target])
    connect_cluster_mock = mocker.patch(
        "src.mcp_server.connect_cluster_service",
        return_value=ConnectResult(
            success=True,
            context_name="acme-prod",
            local_port=16443,
            internal_ip="10.0.0.10",
            tunnel_pid=4242,
            used_cache=False,
            network_requirement=NetworkRequirement.none(),
        ),
    )

    server = build_mcp_server(project_dir=tmp_path)

    result = asyncio.run(
        server.call_tool("connect_cluster", {"context_name": "acme-prod"})
    )

    assert result.structured_content == {
        "success": True,
        "context_name": "acme-prod",
        "local_port": 16443,
        "internal_ip": "10.0.0.10",
        "tunnel_pid": 4242,
        "used_cache": False,
        "network_requirement": {
            "type": None,
            "network_range": None,
            "needs_vpn": False,
        },
        "error": None,
    }
    connect_cluster_mock.assert_called_once_with(
        target=target,
        config=config,
        allow_manual_network=True,
    )


def test_connect_multiple_requires_explicit_contexts(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    mocker.patch("src.mcp_server.load_effective_config", return_value=config)
    connect_multiple_mock = mocker.patch("src.mcp_server.connect_multiple_service")

    server = build_mcp_server(project_dir=tmp_path)

    result_empty = asyncio.run(
        server.call_tool("connect_multiple", {"context_names": []})
    )
    result_missing = asyncio.run(server.call_tool("connect_multiple"))

    expected = {
        "success": False,
        "results": [],
        "missing_contexts": [],
        "error": {
            "code": "context_names_required",
            "message": "At least one context_name must be provided",
            "retryable": False,
        },
    }
    assert result_empty.structured_content == expected
    assert result_missing.structured_content == expected
    connect_multiple_mock.assert_not_called()


def test_connect_multiple_deduplicates_context_names_preserving_order(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    first = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )
    second = ClusterTarget(
        company="beta",
        host_alias="dev",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.11"},
    )
    mocker.patch("src.mcp_server.load_effective_config", return_value=config)
    mocker.patch("src.mcp_server.list_cluster_targets", return_value=[first, second])
    connect_multiple_mock = mocker.patch(
        "src.mcp_server.connect_multiple_service",
        return_value=[],
    )

    server = build_mcp_server(project_dir=tmp_path)

    result = asyncio.run(
        server.call_tool(
            "connect_multiple",
            {
                "context_names": [
                    "beta-dev",
                    "acme-prod",
                    "beta-dev",
                    "acme-prod",
                ]
            },
        )
    )

    assert result.structured_content == {
        "success": True,
        "results": [],
        "missing_contexts": [],
        "error": None,
    }
    connect_multiple_mock.assert_called_once_with(
        targets=[second, first],
        config=config,
        allow_manual_network=True,
    )


def test_connect_multiple_fails_when_all_context_names_are_invalid(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    mocker.patch("src.mcp_server.load_effective_config", return_value=config)
    mocker.patch("src.mcp_server.list_cluster_targets", return_value=[])
    connect_multiple_mock = mocker.patch("src.mcp_server.connect_multiple_service")

    server = build_mcp_server(project_dir=tmp_path)

    result = asyncio.run(
        server.call_tool("connect_multiple", {"context_names": ["nao-existe"]})
    )

    assert result.structured_content == {
        "success": False,
        "results": [],
        "missing_contexts": ["nao-existe"],
        "error": {
            "code": "no_valid_contexts",
            "message": "No valid cluster contexts were found in inventory",
            "retryable": False,
        },
    }
    connect_multiple_mock.assert_not_called()


def test_kill_tunnel_returns_structured_error_on_failure(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch(
        "src.mcp_server.kill_tunnel_service",
        side_effect=RuntimeError("boom"),
    )

    server = build_mcp_server(project_dir=tmp_path)

    result = asyncio.run(server.call_tool("kill_tunnel", {"context_name": "acme-prod"}))

    assert result.structured_content == {
        "success": False,
        "context_name": "acme-prod",
        "error": {
            "code": "kill_tunnel_failed",
            "message": "Failed to stop tunnel",
            "retryable": False,
        },
    }


def test_config_resource_returns_json(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch(
        "src.mcp_server.load_effective_config",
        return_value=build_config(tmp_path),
    )
    mocker.patch("src.mcp_server.list_cluster_targets", return_value=[])
    mocker.patch("src.mcp_server.list_context_status", return_value=[])

    server = build_mcp_server(project_dir=tmp_path)

    result = asyncio.run(server.read_resource("config://effective"))
    payload = json.loads(result.contents[0].content)

    assert payload == {
        "inventory_path": str(tmp_path / "inventory"),
        "ssh_config_path": "~/.ssh/config",
        "ssh_key_path": "~/.ssh/id_ed25519",
        "remote_k3s_config_path": "/etc/rancher/k3s/k3s.yaml",
        "k3s_api_port": 6443,
        "port_range_start": 16443,
        "port_range_size": 10000,
    }
