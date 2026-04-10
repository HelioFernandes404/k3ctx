"""Unit tests for shared JSON serializers."""

from __future__ import annotations

from src.domain.discovery import (
    ClientPage,
    ClientSummary,
    HostPage,
    HostQuery,
    HostRecord,
    HostResolutionResult,
    PageInfo,
)
from src.interfaces.serialization import (
    client_page_payload,
    host_page_payload,
    host_resolution_payload,
)


def test_client_page_payload_uses_public_keys() -> None:
    payload = client_page_payload(
        ClientPage(
            items=(ClientSummary(client="acme", host_count=2),),
            page=PageInfo(limit=20, returned=1, total=1, has_more=False, next_cursor=None),
        )
    )

    assert payload == {
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


def test_host_page_payload_uses_public_host_fields() -> None:
    payload = host_page_payload(
        HostPage(
            items=(
                HostRecord(
                    client="acme",
                    host_name="api-prod",
                    systemframe_id="sf-1042",
                    addr_ip="10.0.0.10",
                    context_name="acme-api-prod",
                    group="k3s_cluster",
                ),
            ),
            page=PageInfo(limit=20, returned=1, total=1, has_more=False, next_cursor=None),
            query=HostQuery(client="acme", host_name="api"),
        )
    )

    assert payload["items"] == [
        {
            "client": "acme",
            "host_name": "api-prod",
            "systemframe_id": "sf-1042",
            "addr_ip": "10.0.0.10",
            "context_name": "acme-api-prod",
            "group": "k3s_cluster",
        }
    ]
    assert payload["query"] == {
        "client": "acme",
        "host_name": "api",
        "systemframe_id": None,
        "addr_ip": None,
        "context_name": None,
        "query": None,
        "exact": False,
    }


def test_host_resolution_payload_includes_hint_and_suggested_commands() -> None:
    payload = host_resolution_payload(
        HostResolutionResult(
            status="ambiguous",
            query=HostQuery(client="acme", host_name="api"),
            matches=(
                HostRecord(
                    client="acme",
                    host_name="api-prod",
                    systemframe_id="sf-1042",
                    addr_ip="10.0.0.10",
                    context_name="acme-api-prod",
                    group="k3s_cluster",
                ),
            ),
            page=PageInfo(limit=10, returned=1, total=2, has_more=True, next_cursor="acme-api-prod"),
            context_name=None,
            hint="Multiple hosts matched. Refine with --ip, --id, or a more specific host name.",
        ),
        cli_name="context-tunnel-manager",
    )

    assert payload["error"]["code"] == "ambiguous_target"
    assert payload["hint"] == "Multiple hosts matched. Refine with --ip, --id, or a more specific host name."
    assert payload["suggested_commands"] == [
        "context-tunnel-manager hosts acme --host api-prod",
        "context-tunnel-manager connect --ip 10.0.0.10",
        "context-tunnel-manager connect --id sf-1042",
    ]
