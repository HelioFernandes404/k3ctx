"""Shared runtime helpers for context and tunnel status."""

from __future__ import annotations

import os
import subprocess
from pathlib import Path

from src.config import load_effective_config
from src.models import EffectiveConfig
from src.tunnel import TUNNEL_STATE_DIR, get_tunnel_pid_file, get_unique_port

PROJECT_DIR = Path(__file__).resolve().parent.parent


def get_current_context() -> str | None:
    """Return the current kubectl context, if available."""
    try:
        result = subprocess.run(
            ["kubectl", "config", "current-context"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except (subprocess.TimeoutExpired, FileNotFoundError):
        pass
    return None


def get_tunnel_pid(
    context_name: str,
    state_dir: Path | None = None,
) -> int | None:
    """Return the running tunnel PID for a context, if present."""
    resolved_state_dir = state_dir or TUNNEL_STATE_DIR
    pid_file = get_tunnel_pid_file(context_name, resolved_state_dir)
    if not pid_file.exists():
        return None

    try:
        pid = int(pid_file.read_text().strip())
        os.kill(pid, 0)
        return pid
    except (ValueError, ProcessLookupError, OSError):
        return None


def load_status_config() -> EffectiveConfig:
    """Load canonical config used by status views."""
    config_path = os.getenv("CONFIG_FILE")
    return load_effective_config(PROJECT_DIR, config_path)


def get_tunnel_port(
    context_name: str,
    config: EffectiveConfig | None = None,
) -> int:
    """Resolve the deterministic local port for a context."""
    resolved_config = config or load_status_config()
    return get_unique_port(
        context_name,
        resolved_config.port_range_start,
        resolved_config.port_range_size,
    )


def list_all_context_names(state_dir: Path | None = None) -> list[str]:
    """List known contexts from tunnel/network state files."""
    resolved_state_dir = state_dir or TUNNEL_STATE_DIR
    if not resolved_state_dir.exists():
        return []

    context_names = {
        file_path.stem
        for pattern in ("*.pid", "*.network")
        for file_path in resolved_state_dir.glob(pattern)
    }
    return sorted(context_names)
