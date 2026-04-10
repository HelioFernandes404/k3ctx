"""Status-related application use cases."""

from __future__ import annotations

from src.application.ports import StatusReader


def list_context_status(reader: StatusReader) -> list[dict[str, object]]:
    return reader.list_context_status()


def validate_context_network(
    context_name: str,
    reader: StatusReader,
) -> dict[str, object]:
    return reader.validate_context_network(context_name)
