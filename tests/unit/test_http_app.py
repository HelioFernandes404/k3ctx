"""Unit tests for the HTTP interface."""

from __future__ import annotations

import asyncio
from pathlib import Path
from types import SimpleNamespace
from typing import Any, cast

import httpx
from pytest_mock import MockerFixture

from src.bootstrap import ServiceContainer
from src.domain.models import ClusterTarget, ConnectResult, EffectiveConfig, NetworkRequirement
from src.interfaces.http.app import build_http_app


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


def build_target() -> ClusterTarget:
    return ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "203.0.113.10"},
        group_vars={},
    )


def build_services() -> ServiceContainer:
    return cast(
        ServiceContainer,
        SimpleNamespace(
            catalog=object(),
            connector=object(),
            switcher=object(),
            status_reader=object(),
            tunnel_manager=object(),
        ),
    )


def build_connect_result() -> ConnectResult:
    return ConnectResult(
        success=True,
        context_name="acme-prod",
        local_port=16443,
        internal_ip="10.0.0.10",
        tunnel_pid=4242,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
    )


def request(
    app: Any,
    method: str,
    path: str,
    *,
    json: dict[str, object] | None = None,
) -> httpx.Response:
    async def run() -> httpx.Response:
        transport = httpx.ASGITransport(app=app)
        async with httpx.AsyncClient(
            transport=transport,
            base_url="http://testserver",
        ) as client:
            return await client.request(method, path, json=json)

    return asyncio.run(run())


def test_get_config_returns_effective_config_payload(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch("src.interfaces.http.app.load_effective_config", return_value=build_config(tmp_path))

    app = build_http_app(project_dir=tmp_path, services=build_services())
    response = request(app, "GET", "/config")

    assert response.status_code == 200
    assert response.json() == {
        "inventory_path": str(tmp_path / "inventory"),
        "ssh_config_path": "~/.ssh/config",
        "ssh_key_path": "~/.ssh/id_ed25519",
        "remote_k3s_config_path": "/etc/rancher/k3s/k3s.yaml",
        "k3s_api_port": 6443,
        "port_range_start": 16443,
        "port_range_size": 10000,
    }


def test_get_clusters_returns_cluster_payloads(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = build_target()
    services = build_services()
    mocker.patch("src.interfaces.http.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.http.app.list_cluster_targets", return_value=[target])

    app = build_http_app(project_dir=tmp_path, services=services)
    response = request(app, "GET", "/clusters")

    assert response.status_code == 200
    assert response.json() == [
        {
            "company": "acme",
            "host_alias": "prod",
            "group": "k3s_cluster",
            "context_name": "acme-prod",
            "host_config": {"ansible_host": "203.0.113.10"},
            "group_vars": {},
        }
    ]


def test_get_status_returns_context_status_list(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    services = build_services()
    mocker.patch("src.interfaces.http.app.load_effective_config", return_value=build_config(tmp_path))
    mocker.patch(
        "src.interfaces.http.app.list_context_status",
        return_value=[
            {
                "name": "acme-prod",
                "is_current": True,
                "tunnel_running": True,
                "tunnel_pid": 4242,
                "local_port": 16443,
                "network_metadata": None,
            }
        ],
    )

    app = build_http_app(project_dir=tmp_path, services=services)
    response = request(app, "GET", "/status")

    assert response.status_code == 200
    assert response.json() == [
        {
            "name": "acme-prod",
            "is_current": True,
            "tunnel_running": True,
            "tunnel_pid": 4242,
            "local_port": 16443,
            "network_metadata": None,
        }
    ]


def test_post_connect_returns_same_payload_as_mcp_connect_cluster(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = build_target()
    services = build_services()
    mocker.patch("src.interfaces.http.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.http.app.find_target_by_context_name", return_value=target)
    connect_cluster = mocker.patch(
        "src.interfaces.http.app.connect_cluster_use_case",
        return_value=build_connect_result(),
    )

    app = build_http_app(project_dir=tmp_path, services=services)
    response = request(app, "POST", "/connect", json={"context_name": "acme-prod"})

    assert response.status_code == 200
    assert response.json() == build_connect_result().to_public_dict()
    connect_cluster.assert_called_once_with(
        target=target,
        config=config,
        connector=services.connector,
        allow_manual_network=True,
    )


def test_post_connect_returns_context_not_found_payload(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    services = build_services()
    mocker.patch("src.interfaces.http.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.http.app.find_target_by_context_name", return_value=None)

    app = build_http_app(project_dir=tmp_path, services=services)
    response = request(app, "POST", "/connect", json={"context_name": "missing"})

    assert response.status_code == 404
    assert response.json() == {
        "success": False,
        "context_name": "missing",
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


def test_post_contexts_current_returns_same_payload_as_mcp(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    services = build_services()
    mocker.patch("src.interfaces.http.app.load_effective_config", return_value=build_config(tmp_path))
    set_current_context = mocker.patch(
        "src.interfaces.http.app.set_current_context_use_case",
        return_value=None,
    )

    app = build_http_app(project_dir=tmp_path, services=services)
    response = request(
        app,
        "POST",
        "/contexts/current",
        json={"context_name": "acme-prod", "require_confirmation": False, "confirmed": True},
    )

    assert response.status_code == 200
    assert response.json() == {
        "success": True,
        "context_name": "acme-prod",
        "error": None,
    }
    set_current_context.assert_called_once_with(
        "acme-prod",
        switcher=services.switcher,
        require_confirmation=False,
        confirmed=True,
    )


def test_post_kill_tunnel_returns_same_payload_as_mcp(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    services = build_services()
    mocker.patch("src.interfaces.http.app.load_effective_config", return_value=build_config(tmp_path))
    kill_tunnel = mocker.patch("src.interfaces.http.app.kill_tunnel_use_case")

    app = build_http_app(project_dir=tmp_path, services=services)
    response = request(app, "POST", "/tunnels/acme-prod/kill")

    assert response.status_code == 200
    assert response.json() == {
        "success": True,
        "context_name": "acme-prod",
        "error": None,
    }
    kill_tunnel.assert_called_once_with("acme-prod", services.tunnel_manager)
