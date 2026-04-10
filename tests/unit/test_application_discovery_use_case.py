"""Unit tests for discovery-focused inventory use cases."""

from __future__ import annotations

from pathlib import Path

from src.application.use_cases.discovery import (
    build_host_records,
    list_client_summaries,
    load_host_records,
    resolve_host_records,
    resolve_host,
    search_hosts,
)
from src.domain.discovery import ClientSummary, HostQuery, HostRecord
from src.domain.models import ClusterTarget


class StubCatalog:
    def __init__(self, targets: list[ClusterTarget]) -> None:
        self.targets = targets
        self.calls: list[Path] = []

    def list_targets(self, inventory_path: Path) -> list[ClusterTarget]:
        self.calls.append(inventory_path)
        return list(self.targets)


def build_target(
    *,
    company: str = "acme",
    host_alias: str = "prod",
    addr_ip: str = "10.0.0.10",
    systemframe_id: str | None = None,
    group_systemframe_id: str | None = None,
) -> ClusterTarget:
    host_config: dict[str, object] = {"ansible_host": addr_ip}
    if systemframe_id is not None:
        host_config["systemframe_id"] = systemframe_id

    group_vars: dict[str, object] = {}
    if group_systemframe_id is not None:
        group_vars["systemframe_id"] = group_systemframe_id

    return ClusterTarget(
        company=company,
        host_alias=host_alias,
        group="k3s_cluster",
        host_config=host_config,
        group_vars=group_vars,
    )


def test_load_host_records_projects_public_fields_from_cluster_targets(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [build_target(systemframe_id="sf-1042")]
    )

    records = load_host_records(tmp_path, catalog)

    assert records == [
        HostRecord(
            client="acme",
            host_name="prod",
            systemframe_id="sf-1042",
            addr_ip="10.0.0.10",
            context_name="acme-prod",
            group="k3s_cluster",
        )
    ]
    assert catalog.calls == [tmp_path]


def test_build_host_records_projects_public_fields_from_targets() -> None:
    records = build_host_records([build_target(systemframe_id="sf-1042")])

    assert records == [
        HostRecord(
            client="acme",
            host_name="prod",
            systemframe_id="sf-1042",
            addr_ip="10.0.0.10",
            context_name="acme-prod",
            group="k3s_cluster",
        )
    ]


def test_load_host_records_falls_back_to_group_vars_for_systemframe_id(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [build_target(systemframe_id=None, group_systemframe_id="sf-group-7")]
    )

    records = load_host_records(tmp_path, catalog)

    assert records[0].systemframe_id == "sf-group-7"


def test_list_client_summaries_returns_sorted_counts_and_cursor(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [
            build_target(company="beta", host_alias="api-01"),
            build_target(company="acme", host_alias="api-01"),
            build_target(company="acme", host_alias="api-02"),
        ]
    )

    page = list_client_summaries(tmp_path, catalog, limit=1)

    assert page.items == (
        ClientSummary(client="acme", host_count=2),
    )
    assert page.page.limit == 1
    assert page.page.returned == 1
    assert page.page.total == 2
    assert page.page.has_more is True
    assert page.page.next_cursor == "acme"


def test_search_hosts_filters_inside_one_client_and_uses_context_cursor(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [
            build_target(company="acme", host_alias="api-01", systemframe_id="sf-1"),
            build_target(company="acme", host_alias="api-02", systemframe_id="sf-2"),
            build_target(company="acme", host_alias="db-01", systemframe_id="sf-3"),
            build_target(company="beta", host_alias="api-01", systemframe_id="sf-9"),
        ]
    )

    first_page = search_hosts(
        tmp_path,
        catalog,
        query=HostQuery(client="acme", host_name="api"),
        limit=1,
    )

    assert [item.context_name for item in first_page.items] == ["acme-api-01"]
    assert first_page.page.total == 2
    assert first_page.page.has_more is True
    assert first_page.page.next_cursor == "acme-api-01"

    second_page = search_hosts(
        tmp_path,
        catalog,
        query=HostQuery(client="acme", host_name="api"),
        limit=1,
        cursor=first_page.page.next_cursor,
    )

    assert [item.context_name for item in second_page.items] == ["acme-api-02"]
    assert second_page.page.has_more is False


def test_search_hosts_supports_id_ip_and_free_text_filters(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [
            build_target(
                company="acme",
                host_alias="api-prod",
                addr_ip="10.0.0.10",
                systemframe_id="sf-1042",
            ),
            build_target(
                company="acme",
                host_alias="api-dev",
                addr_ip="10.0.0.11",
                systemframe_id="sf-2001",
            ),
        ]
    )

    page = search_hosts(
        tmp_path,
        catalog,
        query=HostQuery(
            client="acme",
            query="1042",
            addr_ip="10.0.0.10",
        ),
        limit=20,
    )

    assert [item.context_name for item in page.items] == ["acme-api-prod"]
    assert page.page.total == 1


def test_resolve_host_returns_unique_context_for_single_match(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [
            build_target(company="acme", host_alias="api-prod", systemframe_id="sf-1042"),
            build_target(company="acme", host_alias="db-prod", systemframe_id="sf-2001"),
        ]
    )

    result = resolve_host(
        tmp_path,
        catalog,
        query=HostQuery(client="acme", host_name="api"),
    )

    assert result.status == "unique"
    assert result.context_name == "acme-api-prod"
    assert [item.context_name for item in result.matches] == ["acme-api-prod"]


def test_resolve_host_returns_ambiguous_preview_with_hint(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [
            build_target(company="acme", host_alias="api-01", systemframe_id="sf-1"),
            build_target(company="acme", host_alias="api-02", systemframe_id="sf-2"),
            build_target(company="acme", host_alias="api-03", systemframe_id="sf-3"),
        ]
    )

    result = resolve_host(
        tmp_path,
        catalog,
        query=HostQuery(client="acme", host_name="api"),
        limit=2,
    )

    assert result.status == "ambiguous"
    assert result.context_name is None
    assert [item.context_name for item in result.matches] == [
        "acme-api-01",
        "acme-api-02",
    ]
    assert result.page.total == 3
    assert result.hint == "Multiple hosts matched. Refine with --ip, --id, or a more specific host name."


def test_resolve_host_returns_no_match_when_filters_do_not_overlap(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [
            build_target(company="acme", host_alias="api-prod", addr_ip="10.0.0.10"),
            build_target(company="acme", host_alias="db-prod", addr_ip="10.0.0.20"),
        ]
    )

    result = resolve_host(
        tmp_path,
        catalog,
        query=HostQuery(client="acme", host_name="api", addr_ip="10.0.0.20"),
    )

    assert result.status == "no_match"
    assert result.context_name is None
    assert result.matches == ()
    assert result.hint == "No hosts matched the provided identifiers."


def test_resolve_host_records_returns_unique_context_without_catalog_roundtrip() -> None:
    records = build_host_records(
        [
            build_target(company="acme", host_alias="api-prod", systemframe_id="sf-1042"),
            build_target(company="acme", host_alias="db-prod", systemframe_id="sf-2001"),
        ]
    )

    result = resolve_host_records(
        records=records,
        query=HostQuery(client="acme", host_name="api"),
    )

    assert result.status == "unique"
    assert result.context_name == "acme-api-prod"
