"""Discovery-focused application use cases."""

from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Callable, Sequence, TypeVar

from src.application.ports import InventoryCatalog
from src.application.use_cases.inventory import list_cluster_targets
from src.domain.discovery import (
    ClientPage,
    ClientSummary,
    HostPage,
    HostQuery,
    HostRecord,
    HostResolutionResult,
    PageInfo,
)
from src.domain.models import ClusterTarget

T = TypeVar("T")


def _optional_str(value: object) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    return text or None


def _systemframe_id_for(target: ClusterTarget) -> str | None:
    return _optional_str(
        target.host_config.get("systemframe_id")
        or target.group_vars.get("systemframe_id")
    )


def project_target_to_host_record(target: ClusterTarget) -> HostRecord:
    return HostRecord(
        client=target.company,
        host_name=target.host_alias,
        systemframe_id=_systemframe_id_for(target),
        addr_ip=_optional_str(target.host_config.get("ansible_host")),
        context_name=target.context_name,
        group=target.group,
    )


def load_host_records(
    inventory_path: Path,
    catalog: InventoryCatalog,
) -> list[HostRecord]:
    return [
        project_target_to_host_record(target)
        for target in list_cluster_targets(inventory_path, catalog)
    ]


def _paginate(
    items: Sequence[T],
    *,
    limit: int,
    cursor: str | None,
    key_fn: Callable[[T], str],
) -> tuple[tuple[T, ...], PageInfo]:
    normalized_limit = max(1, limit)
    ordered = sorted(items, key=key_fn)
    start_index = 0

    if cursor is not None:
        for index, item in enumerate(ordered):
            if key_fn(item) > cursor:
                start_index = index
                break
        else:
            start_index = len(ordered)

    selected = tuple(ordered[start_index : start_index + normalized_limit])
    has_more = start_index + normalized_limit < len(ordered)

    return selected, PageInfo(
        limit=normalized_limit,
        returned=len(selected),
        total=len(ordered),
        has_more=has_more,
        next_cursor=key_fn(selected[-1]) if has_more and selected else None,
    )


def list_client_summaries(
    inventory_path: Path,
    catalog: InventoryCatalog,
    *,
    query: str | None = None,
    limit: int = 20,
    cursor: str | None = None,
) -> ClientPage:
    counts = Counter(record.client for record in load_host_records(inventory_path, catalog))
    items = [
        ClientSummary(client=client, host_count=host_count)
        for client, host_count in counts.items()
        if query is None or query.lower() in client.lower()
    ]
    page_items, page = _paginate(
        items,
        limit=limit,
        cursor=cursor,
        key_fn=lambda item: item.client,
    )
    return ClientPage(items=page_items, page=page)


def _matches_value(value: str | None, expected: str | None, *, exact: bool) -> bool:
    if expected is None:
        return True
    if value is None:
        return False

    left = value.lower()
    right = expected.lower()
    if exact:
        return left == right
    return left.startswith(right) or right in left


def _matches_query(record: HostRecord, query: HostQuery) -> bool:
    if query.client is not None and record.client != query.client:
        return False
    if not _matches_value(record.host_name, query.host_name, exact=query.exact):
        return False
    if not _matches_value(record.systemframe_id, query.systemframe_id, exact=query.exact):
        return False
    if not _matches_value(record.addr_ip, query.addr_ip, exact=query.exact):
        return False
    if not _matches_value(record.context_name, query.context_name, exact=True):
        return False

    if query.query is None:
        return True

    search_text = query.query.lower()
    haystacks = (
        record.host_name.lower(),
        record.client.lower(),
        record.context_name.lower(),
        (record.systemframe_id or "").lower(),
        (record.addr_ip or "").lower(),
    )
    return any(search_text in haystack for haystack in haystacks)


def search_hosts(
    inventory_path: Path,
    catalog: InventoryCatalog,
    *,
    query: HostQuery,
    limit: int = 20,
    cursor: str | None = None,
) -> HostPage:
    matches = [
        record
        for record in load_host_records(inventory_path, catalog)
        if _matches_query(record, query)
    ]
    page_items, page = _paginate(
        matches,
        limit=limit,
        cursor=cursor,
        key_fn=lambda item: item.context_name,
    )
    return HostPage(items=page_items, page=page, query=query)


def resolve_host(
    inventory_path: Path,
    catalog: InventoryCatalog,
    *,
    query: HostQuery,
    limit: int = 10,
) -> HostResolutionResult:
    page = search_hosts(
        inventory_path,
        catalog,
        query=query,
        limit=limit,
        cursor=None,
    )

    if page.page.total == 1:
        record = page.items[0]
        return HostResolutionResult(
            status="unique",
            query=query,
            matches=page.items,
            page=page.page,
            context_name=record.context_name,
            hint=None,
        )

    if page.page.total == 0:
        return HostResolutionResult(
            status="no_match",
            query=query,
            matches=(),
            page=page.page,
            context_name=None,
            hint="No hosts matched the provided identifiers.",
        )

    return HostResolutionResult(
        status="ambiguous",
        query=query,
        matches=page.items,
        page=page.page,
        context_name=None,
        hint="Multiple hosts matched. Refine with --ip, --id, or a more specific host name.",
    )
