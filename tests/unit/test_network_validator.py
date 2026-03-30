"""Unit tests for network validator error handling."""

from pathlib import Path

from src.network_validator import (
    get_network_metadata,
    validate_context_network,
    validate_context_network_details,
)


def test_corrupted_network_metadata_returns_safe_failure(tmp_path: Path) -> None:
    state_dir = tmp_path / "state"
    state_dir.mkdir()
    (state_dir / "acme-prod.network").write_text("network_type: [broken")

    metadata = get_network_metadata("acme-prod", state_dir)
    ok, warning = validate_context_network("acme-prod", state_dir)
    details = validate_context_network_details("acme-prod", state_dir)

    assert isinstance(metadata, dict)
    assert metadata["corrupted"] is True
    assert ok is False
    assert warning is not None
    assert "could not be read safely" in warning
    assert details["ok"] is False
    assert details["warning"] == warning
    assert details["network_metadata"]["corrupted"] is True
