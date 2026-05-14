"""Tests for status application use cases."""

from __future__ import annotations

from src.application.use_cases.status import list_context_status, validate_context_network


class StubStatusReader:
    def __init__(
        self,
        status_items: list[dict[str, object]] | None = None,
        network_result: dict[str, object] | None = None,
    ) -> None:
        self.status_items = status_items or []
        self.network_result = network_result or {}
        self.list_calls: int = 0
        self.validate_calls: list[str] = []

    def list_context_status(self) -> list[dict[str, object]]:
        self.list_calls += 1
        return self.status_items

    def validate_context_network(self, context_name: str) -> dict[str, object]:
        self.validate_calls.append(context_name)
        return self.network_result


def test_list_context_status_delegates_to_reader() -> None:
    expected: list[dict[str, object]] = [{"name": "acme-prod", "is_current": True}]
    reader = StubStatusReader(status_items=expected)

    result = list_context_status(reader)

    assert result == expected
    assert reader.list_calls == 1


def test_list_context_status_returns_empty_when_no_contexts() -> None:
    reader = StubStatusReader(status_items=[])

    result = list_context_status(reader)

    assert result == []
    assert reader.list_calls == 1


def test_list_context_status_returns_multiple_contexts() -> None:
    items: list[dict[str, object]] = [
        {"name": "acme-prod", "is_current": True, "tunnel_running": True},
        {"name": "acme-dev", "is_current": False, "tunnel_running": False},
    ]
    reader = StubStatusReader(status_items=items)

    result = list_context_status(reader)

    assert len(result) == 2
    assert result[0]["name"] == "acme-prod"
    assert result[1]["name"] == "acme-dev"


def test_validate_context_network_delegates_to_reader() -> None:
    expected: dict[str, object] = {
        "context_name": "acme-prod",
        "ok": True,
        "warning": None,
        "network_metadata": None,
    }
    reader = StubStatusReader(network_result=expected)

    result = validate_context_network("acme-prod", reader)

    assert result == expected
    assert reader.validate_calls == ["acme-prod"]


def test_validate_context_network_passes_context_name_to_reader() -> None:
    reader = StubStatusReader(network_result={"ok": True, "warning": None})

    validate_context_network("beta-staging", reader)

    assert reader.validate_calls == ["beta-staging"]


def test_validate_context_network_returns_reader_failure_response() -> None:
    expected: dict[str, object] = {
        "context_name": "acme-dev",
        "ok": False,
        "warning": "VPN required",
        "network_metadata": {"needs_vpn": True},
    }
    reader = StubStatusReader(network_result=expected)

    result = validate_context_network("acme-dev", reader)

    assert result["ok"] is False
    assert result["warning"] == "VPN required"
    assert reader.validate_calls == ["acme-dev"]
