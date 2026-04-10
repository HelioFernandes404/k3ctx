"""Tests for the official CLI interface."""

from __future__ import annotations

import json
from pathlib import Path

from pytest import CaptureFixture
from pytest_mock import MockerFixture

from src.domain.discovery import (
    ClientPage,
    ClientSummary,
    HostQuery,
    HostRecord,
    HostResolutionResult,
    PageInfo,
)
from src.domain.models import ClusterTarget, ConnectResult, EffectiveConfig, NetworkRequirement
from src.interfaces.cli.app import main


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


def build_success_result(context_name: str = "acme-prod") -> ConnectResult:
    return ConnectResult(
        success=True,
        context_name=context_name,
        local_port=16443,
        internal_ip="10.0.0.10",
        tunnel_pid=4242,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
    )


def test_clients_command_returns_json_payload(
    mocker: MockerFixture,
    tmp_path: Path,
    capsys: CaptureFixture[str],
) -> None:
    config = build_config(tmp_path)
    mocker.patch("src.interfaces.cli.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.cli.app.setup_logging")
    mocker.patch("src.interfaces.cli.app.refresh_inventory_if_possible")
    mocker.patch(
        "src.interfaces.cli.app.list_client_summaries",
        return_value=ClientPage(
            items=(ClientSummary(client="acme", host_count=2),),
            page=PageInfo(limit=20, returned=1, total=1, has_more=False, next_cursor=None),
        ),
    )

    exit_code = main(["clients", "--json"])

    assert exit_code == 0
    assert json.loads(capsys.readouterr().out) == {
        "ok": True,
        "items": [{"client": "acme", "host_count": 2}],
        "pagination": {
            "limit": 20,
            "returned": 1,
            "total": 1,
            "has_more": False,
            "next_cursor": None,
        },
    }


def test_hosts_command_uses_client_scope_and_search_filters(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    mocker.patch("src.interfaces.cli.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.cli.app.setup_logging")
    mocker.patch("src.interfaces.cli.app.refresh_inventory_if_possible")
    search_hosts = mocker.patch("src.interfaces.cli.app.search_hosts")
    mocker.patch("src.interfaces.cli.app.print_host_page")

    exit_code = main(["hosts", "acme", "--host", "api"])

    assert exit_code == 0
    search_hosts.assert_called_once_with(
        config.inventory_path,
        mocker.ANY,
        query=HostQuery(client="acme", host_name="api"),
        limit=20,
        cursor=None,
    )


def test_connect_command_resolves_before_connecting(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = build_target()
    resolution = HostResolutionResult(
        status="unique",
        query=HostQuery(client="acme", host_name="prod"),
        matches=(
            HostRecord(
                client="acme",
                host_name="prod",
                systemframe_id=None,
                addr_ip="203.0.113.10",
                context_name="acme-prod",
                group="k3s_cluster",
            ),
        ),
        page=PageInfo(limit=10, returned=1, total=1, has_more=False, next_cursor=None),
        context_name="acme-prod",
        hint=None,
    )

    mocker.patch("src.interfaces.cli.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.cli.app.setup_logging")
    mocker.patch("src.interfaces.cli.app.refresh_inventory_if_possible")
    mocker.patch("src.interfaces.cli.app.load_host_records", return_value=list(resolution.matches))
    mocker.patch("src.interfaces.cli.app.resolve_host", return_value=resolution)
    mocker.patch("src.interfaces.cli.app.find_target_by_context_name", return_value=target)
    connect_cluster = mocker.patch(
        "src.interfaces.cli.app.connect_cluster",
        return_value=build_success_result(),
    )

    exit_code = main(["connect", "acme", "prod"])

    assert exit_code == 0
    connect_cluster.assert_called_once()


def test_connect_command_rejects_empty_invocation(
    mocker: MockerFixture,
    capsys: CaptureFixture[str],
) -> None:
    mocker.patch("src.interfaces.cli.app.setup_logging")

    exit_code = main(["connect"])

    assert exit_code == 4
    assert "at least one identifier" in capsys.readouterr().err


def test_connect_command_disables_manual_network_flow(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = build_target()
    resolution = HostResolutionResult(
        status="unique",
        query=HostQuery(client="acme", host_name="prod"),
        matches=(
            HostRecord(
                client="acme",
                host_name="prod",
                systemframe_id=None,
                addr_ip="203.0.113.10",
                context_name="acme-prod",
                group="k3s_cluster",
            ),
        ),
        page=PageInfo(limit=10, returned=1, total=1, has_more=False, next_cursor=None),
        context_name="acme-prod",
        hint=None,
    )

    mocker.patch("src.interfaces.cli.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.cli.app.setup_logging")
    mocker.patch("src.interfaces.cli.app.refresh_inventory_if_possible")
    mocker.patch("src.interfaces.cli.app.load_host_records", return_value=list(resolution.matches))
    mocker.patch("src.interfaces.cli.app.resolve_host", return_value=resolution)
    mocker.patch("src.interfaces.cli.app.find_target_by_context_name", return_value=target)
    connect_cluster = mocker.patch(
        "src.interfaces.cli.app.connect_cluster",
        return_value=build_success_result(),
    )

    exit_code = main(["connect", "acme", "prod"])

    assert exit_code == 0
    connect_cluster.assert_called_once_with(
        target=target,
        config=config,
        connector=mocker.ANY,
        allow_manual_network=False,
    )


def test_connect_command_returns_ambiguous_exit_code_without_prompt(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    resolution = HostResolutionResult(
        status="ambiguous",
        query=HostQuery(client="acme", host_name="api"),
        matches=(
            HostRecord(
                client="acme",
                host_name="api-01",
                systemframe_id="sf-1",
                addr_ip="10.0.0.10",
                context_name="acme-api-01",
                group="k3s_cluster",
            ),
        ),
        page=PageInfo(limit=10, returned=1, total=2, has_more=True, next_cursor="acme-api-01"),
        context_name=None,
        hint="Multiple hosts matched. Refine with --ip, --id, or a more specific host name.",
    )

    mocker.patch("src.interfaces.cli.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.cli.app.setup_logging")
    mocker.patch("src.interfaces.cli.app.refresh_inventory_if_possible")
    mocker.patch("src.interfaces.cli.app.load_host_records", return_value=list(resolution.matches))
    mocker.patch("src.interfaces.cli.app.resolve_host", return_value=resolution)
    print_failure = mocker.patch("src.interfaces.cli.app.print_host_resolution_failure")

    exit_code = main(["connect", "acme", "api"])

    assert exit_code == 3
    print_failure.assert_called_once()


def test_single_command_returns_non_interactive_error(
    mocker: MockerFixture,
    capsys: CaptureFixture[str],
) -> None:
    mocker.patch("src.interfaces.cli.app.setup_logging")

    exit_code = main(["single"])

    assert exit_code == 4
    assert "has been removed" in capsys.readouterr().err


def test_multi_command_returns_non_interactive_error(
    mocker: MockerFixture,
    capsys: CaptureFixture[str],
) -> None:
    mocker.patch("src.interfaces.cli.app.setup_logging")

    exit_code = main(["multi"])

    assert exit_code == 4
    assert "has been removed" in capsys.readouterr().err
