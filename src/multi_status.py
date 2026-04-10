"""Compatibility wrapper for legacy multi-status output."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from src.interfaces.cli.presenters import print_status
from src.services.status import list_context_status, validate_context_network
from src.tunnel import TUNNEL_STATE_DIR

ContextStatus = dict[str, Any]


def list_all_contexts(state_dir: Path | None = None) -> list[ContextStatus]:
    """Return legacy status items while delegating reads to shared status services."""
    resolved_state_dir = state_dir or TUNNEL_STATE_DIR
    contexts: list[ContextStatus] = []

    for item in list_context_status(resolved_state_dir):
        context = dict(item)
        if not context.get("tunnel_running"):
            context["tunnel_pid"] = None
        context_name = str(context["name"])
        context["network_validation"] = validate_context_network(
            context_name,
            resolved_state_dir,
        )
        contexts.append(context)

    contexts.sort(key=lambda context: str(context["name"]))
    return contexts


def show_status(state_dir: Path | None = None) -> None:
    """Render the legacy status view using the shared CLI presenter."""
    contexts = list_all_contexts(state_dir)
    items = []
    validations: dict[str, dict[str, Any]] = {}

    for context in contexts:
        item = dict(context)
        validation = item.pop("network_validation", {})
        items.append(item)
        validations[str(item["name"])] = dict(validation) if isinstance(validation, dict) else {}

    print_status(items, validations)


if __name__ == "__main__":
    show_status()
