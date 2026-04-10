# CLI Discovery And Connect Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a discovery-first CLI with `clients`, `hosts`, and `connect`, keeping client-first exploration, JSON output, pagination, ambiguity handling, and compatibility wrappers for the current `single` and `multi` commands.

**Architecture:** Add a read-only discovery layer that projects `ClusterTarget` into public `HostRecord` values, then reuse that layer for client summaries, host search, and connect resolution. Keep the existing tunnel, kubeconfig, and SSH connection path unchanged by resolving to `context_name` first and then reusing the current `find_target_by_context_name()` and `connect_cluster()` flow.

**Tech Stack:** Python 3.10, argparse, pytest, existing application/use_cases layer, existing CLI presenters, existing JSON serialization helpers

---

## File Structure

- Create: `src/domain/discovery.py`
  Responsibility: public discovery/read models used by the new CLI and shared serializers.
- Create: `src/application/use_cases/discovery.py`
  Responsibility: project `ClusterTarget` values into public records, paginate, filter, and resolve unique vs ambiguous matches.
- Modify: `src/domain/__init__.py`
  Responsibility: export the new discovery models.
- Modify: `src/interfaces/serialization.py`
  Responsibility: JSON payload helpers for `clients`, `hosts`, and resolution failures.
- Modify: `src/interfaces/cli/presenters.py`
  Responsibility: human-readable output for client lists, host lists, and ambiguous/no-match connect responses.
- Modify: `src/interfaces/cli/app.py`
  Responsibility: new command tree, dynamic CLI name, JSON output, legacy aliases, and connect resolution before calling the existing connect use case.
- Modify: `pyproject.toml`
  Responsibility: add the neutral console alias `context-tunnel-manager` while keeping the current script as a compatibility alias.
- Modify: `Makefile`
  Responsibility: move `make run` to the new `connect` entrypoint and mark `multi-connect` as legacy.
- Modify: `README.md`
  Responsibility: document the new command flow, JSON output, and legacy aliases.
- Create: `tests/unit/test_application_discovery_use_case.py`
  Responsibility: discovery projection, filtering, pagination, and resolution tests.
- Create: `tests/unit/test_interfaces_serialization.py`
  Responsibility: JSON payload coverage for the new public CLI outputs.
- Modify: `tests/unit/test_cli_app.py`
  Responsibility: command-level behavior for `clients`, `hosts`, `connect`, `status --json`, and legacy aliases.

### Task 1: Add Discovery Read Models And Host Projection

**Files:**
- Create: `src/domain/discovery.py`
- Create: `src/application/use_cases/discovery.py`
- Modify: `src/domain/__init__.py`
- Test: `tests/unit/test_application_discovery_use_case.py`

- [ ] **Step 1: Write the failing projection tests**

Add this new test file:

```python
"""Unit tests for discovery-focused inventory use cases."""

from __future__ import annotations

from pathlib import Path

from src.domain.discovery import HostRecord
from src.domain.models import ClusterTarget
from src.application.use_cases.discovery import load_host_records


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


def test_load_host_records_falls_back_to_group_vars_for_systemframe_id(
    tmp_path: Path,
) -> None:
    catalog = StubCatalog(
        [build_target(systemframe_id=None, group_systemframe_id="sf-group-7")]
    )

    records = load_host_records(tmp_path, catalog)

    assert records[0].systemframe_id == "sf-group-7"
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
uv run python -m pytest tests/unit/test_application_discovery_use_case.py -q
```

Expected:

```text
FAIL ... ModuleNotFoundError: No module named 'src.domain.discovery'
```

- [ ] **Step 3: Write the minimal discovery models and projection helper**

Create `src/domain/discovery.py`:

```python
"""Read-only discovery models for the CLI and other machine-readable interfaces."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Literal


@dataclass(frozen=True)
class HostRecord:
    client: str
    host_name: str
    systemframe_id: str | None
    addr_ip: str | None
    context_name: str
    group: str


@dataclass(frozen=True)
class ClientSummary:
    client: str
    host_count: int


@dataclass(frozen=True)
class PageInfo:
    limit: int
    returned: int
    total: int
    has_more: bool
    next_cursor: str | None


@dataclass(frozen=True)
class HostQuery:
    client: str | None = None
    host_name: str | None = None
    systemframe_id: str | None = None
    addr_ip: str | None = None
    context_name: str | None = None
    query: str | None = None
    exact: bool = False


@dataclass(frozen=True)
class ClientPage:
    items: tuple[ClientSummary, ...]
    page: PageInfo


@dataclass(frozen=True)
class HostPage:
    items: tuple[HostRecord, ...]
    page: PageInfo
    query: HostQuery


@dataclass(frozen=True)
class HostResolutionResult:
    status: Literal["unique", "no_match", "ambiguous"]
    query: HostQuery
    matches: tuple[HostRecord, ...]
    page: PageInfo
    context_name: str | None
    hint: str | None
```

Create `src/application/use_cases/discovery.py`:

```python
"""Discovery-focused application use cases."""

from __future__ import annotations

from pathlib import Path

from src.application.ports import InventoryCatalog
from src.application.use_cases.inventory import list_cluster_targets
from src.domain.discovery import HostRecord
from src.domain.models import ClusterTarget


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
```

Update `src/domain/__init__.py`:

```python
"""Core domain objects and pure policies."""

from src.domain.discovery import (
    ClientPage,
    ClientSummary,
    HostPage,
    HostQuery,
    HostRecord,
    HostResolutionResult,
    PageInfo,
)
from src.domain.models import (
    ClusterTarget,
    ConnectResult,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)

__all__ = [
    "ClientPage",
    "ClientSummary",
    "ClusterTarget",
    "ConnectResult",
    "EffectiveConfig",
    "HostPage",
    "HostQuery",
    "HostRecord",
    "HostResolutionResult",
    "NetworkRequirement",
    "OperationError",
    "PageInfo",
]
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
uv run python -m pytest tests/unit/test_application_discovery_use_case.py -q
```

Expected:

```text
2 passed
```

- [ ] **Step 5: Commit**

```bash
git add src/domain/discovery.py src/application/use_cases/discovery.py src/domain/__init__.py tests/unit/test_application_discovery_use_case.py
git commit -m "feat: add discovery host projection models"
```

### Task 2: Add Client Listing, Host Search, And Cursor Pagination

**Files:**
- Modify: `src/application/use_cases/discovery.py`
- Modify: `tests/unit/test_application_discovery_use_case.py`
- Test: `tests/unit/test_application_discovery_use_case.py`

- [ ] **Step 1: Write the failing list and search tests**

Append these tests to `tests/unit/test_application_discovery_use_case.py`:

```python
from src.domain.discovery import ClientSummary, HostQuery
from src.application.use_cases.discovery import list_client_summaries, search_hosts


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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
uv run python -m pytest tests/unit/test_application_discovery_use_case.py -q
```

Expected:

```text
FAIL ... cannot import name 'list_client_summaries'
```

- [ ] **Step 3: Implement pagination, client summaries, and host filtering**

Update `src/application/use_cases/discovery.py`:

```python
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
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
uv run python -m pytest tests/unit/test_application_discovery_use_case.py -q
```

Expected:

```text
5 passed
```

- [ ] **Step 5: Commit**

```bash
git add src/application/use_cases/discovery.py tests/unit/test_application_discovery_use_case.py
git commit -m "feat: add client and host discovery queries"
```

### Task 3: Add Connect Resolution With Unique, No-Match, And Ambiguous Results

**Files:**
- Modify: `src/application/use_cases/discovery.py`
- Modify: `tests/unit/test_application_discovery_use_case.py`
- Test: `tests/unit/test_application_discovery_use_case.py`

- [ ] **Step 1: Write the failing resolution tests**

Append these tests to `tests/unit/test_application_discovery_use_case.py`:

```python
from src.application.use_cases.discovery import resolve_host


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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
uv run python -m pytest tests/unit/test_application_discovery_use_case.py -q
```

Expected:

```text
FAIL ... cannot import name 'resolve_host'
```

- [ ] **Step 3: Implement resolution outcomes on top of host search**

Update `src/application/use_cases/discovery.py`:

```python
from src.domain.discovery import (
    ClientPage,
    ClientSummary,
    HostPage,
    HostQuery,
    HostRecord,
    HostResolutionResult,
    PageInfo,
)


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
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
uv run python -m pytest tests/unit/test_application_discovery_use_case.py -q
```

Expected:

```text
8 passed
```

- [ ] **Step 5: Commit**

```bash
git add src/application/use_cases/discovery.py tests/unit/test_application_discovery_use_case.py
git commit -m "feat: add host resolution outcomes"
```

### Task 4: Add JSON Payloads And Human Presenters For Discovery Commands

**Files:**
- Create: `tests/unit/test_interfaces_serialization.py`
- Modify: `src/interfaces/serialization.py`
- Modify: `src/interfaces/cli/presenters.py`
- Test: `tests/unit/test_interfaces_serialization.py`

- [ ] **Step 1: Write the failing serializer tests**

Create `tests/unit/test_interfaces_serialization.py`:

```python
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
uv run python -m pytest tests/unit/test_interfaces_serialization.py -q
```

Expected:

```text
FAIL ... cannot import name 'client_page_payload'
```

- [ ] **Step 3: Implement shared payload helpers and CLI presenters**

Update `src/interfaces/serialization.py`:

```python
from src.domain.discovery import (
    ClientPage,
    ClientSummary,
    HostPage,
    HostQuery,
    HostRecord,
    HostResolutionResult,
    PageInfo,
)


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
```

Update `src/interfaces/cli/presenters.py`:

```python
from src.domain.discovery import ClientPage, HostPage, HostResolutionResult


def print_client_page(page: ClientPage) -> None:
    if not page.items:
        print("No clients found.")
        return

    print("CLIENT".ljust(28) + "HOSTS")
    for item in page.items:
        print(f"{item.client:<28}{item.host_count}")

    print(f"\nShowing {page.page.returned} of {page.page.total}")
    if page.page.has_more and page.page.next_cursor is not None:
        print(f"Next page: --cursor {page.page.next_cursor}")


def print_host_page(page: HostPage) -> None:
    if not page.items:
        print("No hosts found.")
        return

    print("HOST NAME".ljust(24) + "SYSTEMFRAME ID".ljust(20) + "IP".ljust(18) + "GROUP")
    for item in page.items:
        print(
            f"{item.host_name:<24}"
            f"{(item.systemframe_id or '-'): <20}"
            f"{(item.addr_ip or '-'): <18}"
            f"{item.group}"
        )

    print(f"\nShowing {page.page.returned} of {page.page.total}")
    if page.page.has_more and page.page.next_cursor is not None:
        print(f"Next page: --cursor {page.page.next_cursor}")


def print_host_resolution_failure(
    result: HostResolutionResult,
    *,
    cli_name: str,
) -> None:
    print(result.hint or "Host resolution failed.")

    if not result.matches:
        return

    print()
    print("HOST NAME".ljust(24) + "SYSTEMFRAME ID".ljust(20) + "IP".ljust(18) + "CONTEXT")
    for item in result.matches:
        print(
            f"{item.host_name:<24}"
            f"{(item.systemframe_id or '-'): <20}"
            f"{(item.addr_ip or '-'): <18}"
            f"{item.context_name}"
        )

    if (
        result.page.has_more
        and result.page.next_cursor is not None
        and result.query.client is not None
    ):
        print(
            f"\nNext page: {cli_name} hosts "
            f"{result.query.client} --cursor {result.page.next_cursor}"
        )
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
uv run python -m pytest tests/unit/test_interfaces_serialization.py -q
```

Expected:

```text
3 passed
```

- [ ] **Step 5: Commit**

```bash
git add src/interfaces/serialization.py src/interfaces/cli/presenters.py tests/unit/test_interfaces_serialization.py
git commit -m "feat: add discovery serializers and presenters"
```

### Task 5: Replace The CLI Surface With Discovery Commands And Legacy Wrappers

**Files:**
- Modify: `src/interfaces/cli/app.py`
- Modify: `tests/unit/test_cli_app.py`
- Modify: `pyproject.toml`
- Test: `tests/unit/test_cli_app.py`

- [ ] **Step 1: Write the failing CLI command tests**

Replace `tests/unit/test_cli_app.py` with:

```python
"""Tests for the official CLI interface."""

from __future__ import annotations

import json
from pathlib import Path

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
    capsys: object,
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
    mocker.patch("src.interfaces.cli.app.show_manual_network_warnings", return_value=True)
    connect_cluster = mocker.patch(
        "src.interfaces.cli.app.connect_cluster",
        return_value=build_success_result(),
    )

    exit_code = main(["connect", "acme", "prod"])

    assert exit_code == 0
    connect_cluster.assert_called_once()


def test_connect_command_rejects_empty_non_interactive_invocation(
    mocker: MockerFixture,
    capsys: object,
) -> None:
    mocker.patch("src.interfaces.cli.app.setup_logging")
    mocker.patch("src.interfaces.cli.app._has_tty", return_value=False)

    exit_code = main(["connect"])

    assert exit_code == 4
    assert "at least one identifier" in capsys.readouterr().err


def test_connect_json_disables_manual_network_prompts(
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
    mocker.patch("src.interfaces.cli.app._has_tty", return_value=False)
    prompt_warning = mocker.patch("src.interfaces.cli.app.show_manual_network_warnings")
    connect_cluster = mocker.patch(
        "src.interfaces.cli.app.connect_cluster",
        return_value=build_success_result(),
    )

    exit_code = main(["connect", "--json", "acme", "prod"])

    assert exit_code == 0
    prompt_warning.assert_not_called()
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


def test_single_command_keeps_guided_flow_compatibility(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = build_target()
    mocker.patch("src.interfaces.cli.app.load_effective_config", return_value=config)
    mocker.patch("src.interfaces.cli.app.setup_logging")
    mocker.patch("src.interfaces.cli.app.refresh_inventory_if_possible")
    mocker.patch("src.interfaces.cli.app.list_cluster_targets", return_value=[target])
    mocker.patch("src.interfaces.cli.app.select_company", return_value="acme")
    mocker.patch("src.interfaces.cli.app.select_single_target", return_value=target)
    mocker.patch("src.interfaces.cli.app.show_manual_network_warnings", return_value=True)
    connect_cluster = mocker.patch(
        "src.interfaces.cli.app.connect_cluster",
        return_value=build_success_result(),
    )

    exit_code = main(["single"])

    assert exit_code == 0
    connect_cluster.assert_called_once()
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
uv run python -m pytest tests/unit/test_cli_app.py -q
```

Expected:

```text
FAIL ... unrecognized arguments: clients --json
```

- [ ] **Step 3: Implement the new command tree, neutral alias, and legacy wrappers**

Update `pyproject.toml`:

```toml
[project.scripts]
context-tunnel-manager = "src.interfaces.cli.app:main"
k3s-context-tunnel-manager = "src.interfaces.cli.app:main"
k3s-context-tunnel-manager-http = "src.interfaces.http.app:main"
```

Update `src/interfaces/cli/app.py`:

```python
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path

from dotenv import load_dotenv

from src.application.use_cases.connect import connect_cluster, connect_multiple
from src.application.use_cases.contexts import set_current_context
from src.application.use_cases.discovery import (
    list_client_summaries,
    load_host_records,
    resolve_host,
    search_hosts,
)
from src.application.use_cases.inventory import (
    find_target_by_context_name,
    list_cluster_targets,
    refresh_inventory_if_possible,
)
from src.application.use_cases.status import list_context_status, validate_context_network
from src.bootstrap import ServiceContainer, build_service_container
from src.config import load_effective_config
from src.domain.discovery import HostQuery
from src.domain.models import EffectiveConfig
from src.interfaces.cli.presenters import (
    print_client_page,
    print_host_page,
    print_host_resolution_failure,
    print_multi_summary,
    print_multi_usage,
    print_network_reminders,
    print_single_failure,
    print_single_success,
    print_status,
    show_manual_network_warnings,
    show_network_warnings,
)
from src.interfaces.cli.prompts import (
    NonInteractiveTerminalError,
    confirm_action,
    select_company,
    select_multiple_targets,
    select_single_target,
)
from src.interfaces.serialization import (
    client_page_payload,
    host_page_payload,
    host_resolution_payload,
    to_jsonable,
)
from src.logging_config import setup_logging

PROJECT_DIR = Path(__file__).resolve().parents[3]
_IP_PATTERN = re.compile(r"^\d{1,3}(?:\.\d{1,3}){3}$")


def _cli_name() -> str:
    return Path(sys.argv[0]).name or "context-tunnel-manager"


def _configure_logging() -> None:
    log_file_path = os.path.expanduser(
        os.getenv("K9S_LOG_FILE", "~/.local/state/k9s/k9s-config.log")
    )
    setup_logging(log_file=log_file_path, structured=True)


def _load_runtime() -> tuple[EffectiveConfig, ServiceContainer]:
    config = load_effective_config(PROJECT_DIR, os.getenv("CONFIG_FILE"))
    services = build_service_container()
    return config, services


def _has_tty() -> bool:
    stdin_isatty = getattr(sys.stdin, "isatty", lambda: False)()
    stdout_isatty = getattr(sys.stdout, "isatty", lambda: False)()
    return bool(stdin_isatty and stdout_isatty)


def _refresh_inventory(
    config: EffectiveConfig,
    services: ServiceContainer,
    *,
    quiet: bool = False,
) -> None:
    refresh_result = refresh_inventory_if_possible(config.inventory_path, services.refresher)
    if refresh_result is None or not isinstance(refresh_result, tuple) or len(refresh_result) != 2:
        return

    if quiet:
        return

    success, message = refresh_result
    if success:
        print(f"✓ {message}")
    else:
        print(f"⚠️  {message} (continuing with local version)")


def _print_json(payload: object) -> None:
    print(json.dumps(to_jsonable(payload), sort_keys=True))


def _confirm_action_or_none(prompt: str, *, default: bool) -> bool | None:
    try:
        return confirm_action(prompt, default=default)
    except KeyboardInterrupt:
        return None


def _guided_connect() -> int:
    config, services = _load_runtime()
    _refresh_inventory(config, services)
    targets = list_cluster_targets(config.inventory_path, services.catalog)
    if not targets:
        print(f"No inventories found in {config.inventory_path}.", file=sys.stderr)
        return 1

    companies = sorted({target.company for target in targets})
    while True:
        company = select_company(companies)
        if company is None:
            print("Cancelled.")
            return 0

        company_targets = [target for target in targets if target.company == company]
        target = select_single_target(company, company_targets)
        if target is None:
            continue
        if not show_manual_network_warnings(target):
            continue

        result = connect_cluster(
            target=target,
            config=config,
            connector=services.connector,
            allow_manual_network=True,
        )
        if result.success:
            print_single_success(result)
            return 0

        print_single_failure(result)
        retry = _confirm_action_or_none("Try another host?", default=True)
        if retry:
            continue
        if retry is None:
            return 0
        return 1


def run_multi() -> int:
    config, services = _load_runtime()
    _refresh_inventory(config, services)
    print("Loading available clusters...")
    targets = list_cluster_targets(config.inventory_path, services.catalog)
    if not targets:
        print("No clusters found in inventory.", file=sys.stderr)
        return 1

    print(f"Found {len(targets)} clusters in inventory")
    selected = select_multiple_targets(targets)
    if not selected:
        print("\nNo clusters selected. Cancelled.")
        return 0

    if not show_network_warnings(selected):
        print("Cancelled.")
        return 0

    print("\n============================================================")
    print("Connecting to clusters...")
    print("============================================================")
    results = connect_multiple(
        targets=selected,
        config=config,
        connector=services.connector,
        allow_manual_network=True,
    )
    successful = print_multi_summary(results)
    if not successful:
        print("\nNo clusters connected successfully.")
        return 1

    first_context = successful[0].context_name
    print(f"\nSetting active context to: {first_context}")
    context_error = set_current_context(
        first_context,
        switcher=services.switcher,
        require_confirmation=False,
        confirmed=True,
    )
    if context_error is None:
        print(f"✓ Active context: {first_context}")
    else:
        print("⚠️  Failed to set active context (you can set it manually)")

    print_network_reminders(successful)
    print_multi_usage()
    return 0


def _build_connect_query(
    *,
    identifiers: list[str],
    client: str | None,
    host_name: str | None,
    systemframe_id: str | None,
    addr_ip: str | None,
    context_name: str | None,
    known_records: list[object],
) -> HostQuery:
    if len(identifiers) > 3:
        raise ValueError("connect accepts at most three identifiers")

    known_clients = {getattr(record, "client") for record in known_records}
    known_contexts = {getattr(record, "context_name") for record in known_records}

    resolved_client = client
    resolved_host = host_name
    resolved_id = systemframe_id
    resolved_ip = addr_ip
    resolved_context = context_name

    for token in identifiers:
        if _IP_PATTERN.match(token) and resolved_ip is None:
            resolved_ip = token
        elif token in known_contexts and resolved_context is None:
            resolved_context = token
        elif token in known_clients and resolved_client is None:
            resolved_client = token
        elif resolved_host is None:
            resolved_host = token
        elif resolved_id is None:
            resolved_id = token
        else:
            raise ValueError("Unable to infer identifier roles. Use explicit flags.")

    return HostQuery(
        client=resolved_client,
        host_name=resolved_host,
        systemframe_id=resolved_id,
        addr_ip=resolved_ip,
        context_name=resolved_context,
    )


def run_clients(args: argparse.Namespace) -> int:
    config, services = _load_runtime()
    _refresh_inventory(config, services, quiet=args.json)
    page = list_client_summaries(
        config.inventory_path,
        services.catalog,
        query=args.query,
        limit=args.limit,
        cursor=args.cursor,
    )
    if args.json:
        _print_json(client_page_payload(page))
    else:
        print_client_page(page)
    return 0


def run_hosts(args: argparse.Namespace) -> int:
    config, services = _load_runtime()
    _refresh_inventory(config, services, quiet=args.json)
    page = search_hosts(
        config.inventory_path,
        services.catalog,
        query=HostQuery(
            client=args.client,
            host_name=args.host,
            systemframe_id=args.systemframe_id,
            addr_ip=args.addr_ip,
            query=args.query,
        ),
        limit=args.limit,
        cursor=args.cursor,
    )
    if args.json:
        _print_json(host_page_payload(page))
    else:
        print_host_page(page)
    return 0


def run_connect(args: argparse.Namespace) -> int:
    if not args.identifiers and not any(
        [args.client, args.host, args.systemframe_id, args.addr_ip, args.context_name]
    ):
        if not _has_tty():
            print(
                "Error: connect requires at least one identifier in non-interactive mode.",
                file=sys.stderr,
            )
            return 4
        return _guided_connect()

    config, services = _load_runtime()
    _refresh_inventory(config, services, quiet=args.json)
    records = load_host_records(config.inventory_path, services.catalog)
    query = _build_connect_query(
        identifiers=args.identifiers,
        client=args.client,
        host_name=args.host,
        systemframe_id=args.systemframe_id,
        addr_ip=args.addr_ip,
        context_name=args.context_name,
        known_records=records,
    )
    resolution = resolve_host(
        config.inventory_path,
        services.catalog,
        query=query,
        limit=10,
    )

    if resolution.status != "unique" or resolution.context_name is None:
        if args.json:
            _print_json(host_resolution_payload(resolution, cli_name=_cli_name()))
        else:
            print_host_resolution_failure(resolution, cli_name=_cli_name())
        return 3 if resolution.status == "ambiguous" else 2

    target = find_target_by_context_name(
        resolution.context_name,
        config.inventory_path,
        services.catalog,
    )
    if target is None:
        print("Resolved context disappeared from inventory.", file=sys.stderr)
        return 1

    allow_manual_network = _has_tty() and not args.json
    if allow_manual_network and not show_manual_network_warnings(target):
        return 1

    result = connect_cluster(
        target=target,
        config=config,
        connector=services.connector,
        allow_manual_network=allow_manual_network,
    )
    if args.json:
        _print_json(result.to_public_dict())
    else:
        if result.success:
            print_single_success(result)
        else:
            print_single_failure(result)
    return 0 if result.success else 1


def run_status(args: argparse.Namespace) -> int:
    _, services = _load_runtime()
    items = list_context_status(services.status_reader)
    if args.json:
        _print_json(items)
        return 0

    validations = {
        str(item["name"]): validate_context_network(str(item["name"]), services.status_reader)
        for item in items
    }
    print_status(items, validations)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog=_cli_name())
    subparsers = parser.add_subparsers(dest="command")

    clients_parser = subparsers.add_parser("clients", help="List clients with host counts")
    clients_parser.add_argument("query", nargs="?")
    clients_parser.add_argument("--limit", type=int, default=20)
    clients_parser.add_argument("--cursor")
    clients_parser.add_argument("--json", action="store_true")

    hosts_parser = subparsers.add_parser("hosts", help="List or search hosts in one client")
    hosts_parser.add_argument("client")
    hosts_parser.add_argument("query", nargs="?")
    hosts_parser.add_argument("--host")
    hosts_parser.add_argument("--id", dest="systemframe_id")
    hosts_parser.add_argument("--ip", dest="addr_ip")
    hosts_parser.add_argument("--limit", type=int, default=20)
    hosts_parser.add_argument("--cursor")
    hosts_parser.add_argument("--json", action="store_true")

    connect_parser = subparsers.add_parser("connect", help="Resolve identifiers and connect if unique")
    connect_parser.add_argument("identifiers", nargs="*")
    connect_parser.add_argument("--client")
    connect_parser.add_argument("--host")
    connect_parser.add_argument("--id", dest="systemframe_id")
    connect_parser.add_argument("--ip", dest="addr_ip")
    connect_parser.add_argument("--context", dest="context_name")
    connect_parser.add_argument("--json", action="store_true")

    status_parser = subparsers.add_parser("status", help="Show active contexts and tunnels")
    status_parser.add_argument("--json", action="store_true")

    subparsers.add_parser("single", help=argparse.SUPPRESS)
    subparsers.add_parser("multi", help=argparse.SUPPRESS)
    return parser


def main(argv: list[str] | None = None) -> int:
    load_dotenv()
    parser = build_parser()
    args = parser.parse_args(argv)
    command = args.command

    _configure_logging()

    try:
        if command in (None, "connect"):
            return run_connect(args if command == "connect" else argparse.Namespace(
                identifiers=[],
                client=None,
                host=None,
                systemframe_id=None,
                addr_ip=None,
                context_name=None,
                json=False,
            ))
        if command == "clients":
            return run_clients(args)
        if command == "hosts":
            return run_hosts(args)
        if command == "single":
            return _guided_connect()
        if command == "multi":
            return run_multi()
        if command == "status":
            return run_status(args)
    except NonInteractiveTerminalError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1
    except ValueError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 4

    parser.print_help()
    return 1
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
uv run python -m pytest tests/unit/test_cli_app.py -q
```

Expected:

```text
7 passed
```

- [ ] **Step 5: Commit**

```bash
git add pyproject.toml src/interfaces/cli/app.py tests/unit/test_cli_app.py
git commit -m "feat: add discovery-oriented CLI commands"
```

### Task 6: Update Operator Docs, Make Targets, And Regression Coverage

**Files:**
- Modify: `Makefile`
- Modify: `README.md`
- Test: `tests/unit/test_application_discovery_use_case.py`
- Test: `tests/unit/test_interfaces_serialization.py`
- Test: `tests/unit/test_cli_app.py`

- [ ] **Step 1: Write the docs and operator changes**

Update `Makefile`:

```makefile
CLI_COMMAND := uv run context-tunnel-manager

## run: Discover and connect to a cluster
run:
	@echo "$(GREEN)Starting Context Tunnel Manager...$(NC)"
	@$(CLI_COMMAND) connect

## multi-connect: Connect to multiple clusters simultaneously (legacy flow)
multi-connect:
	@echo "$(YELLOW)Starting legacy multi-cluster flow...$(NC)"
	@$(CLI_COMMAND) multi

## status: Show status of all connected clusters
status:
	@$(CLI_COMMAND) status
```

Update the CLI usage section in `README.md`:

```markdown
### CLI oficial

Fluxo principal para descoberta e conexao:

```bash
uv run context-tunnel-manager clients
uv run context-tunnel-manager hosts acme
uv run context-tunnel-manager connect acme prod
uv run context-tunnel-manager status
```

Exemplos de refinamento:

```bash
uv run context-tunnel-manager hosts acme --host api --limit 10
uv run context-tunnel-manager connect --ip 10.0.0.10
uv run context-tunnel-manager connect --id sf-1042 --json
```

Aliases legados ainda disponiveis temporariamente:

```bash
uv run context-tunnel-manager single
uv run context-tunnel-manager multi
```

`make run` agora chama `connect`. `make multi-connect` continua disponivel como fluxo legado.
```

- [ ] **Step 2: Run the focused unit suite**

Run:

```bash
uv run python -m pytest tests/unit/test_application_discovery_use_case.py tests/unit/test_interfaces_serialization.py tests/unit/test_cli_app.py -q
```

Expected:

```text
all tests passed
```

- [ ] **Step 3: Run the full verification suite**

Run:

```bash
uv run python -m pytest tests/unit -q
uv run python -m pytest tests/smoke -q
uv run mypy src tests
```

Expected:

```text
unit tests passed
smoke tests passed
Success: no issues found
```

- [ ] **Step 4: Commit**

```bash
git add Makefile README.md
git commit -m "docs: document discovery-first CLI flow"
```
