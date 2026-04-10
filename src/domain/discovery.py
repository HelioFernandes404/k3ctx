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
