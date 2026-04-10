"""Configuration management for k3s-context-tunnel-manager.

Supports loading config from YAML files and merging with environment variables.
Environment variables take precedence over file values.
Numeric values (ports, ranges) are normalized to int type.
"""

import os
from pathlib import Path
from typing import Any, Dict, Optional

import yaml

from .app_paths import get_config_file_path, get_default_config_candidates
from .logging_config import get_logger
from .models import EffectiveConfig

logger = get_logger()

DEFAULT_REMOTE_K3S_CONFIG_PATH = "/etc/rancher/k3s/k3s.yaml"
DEFAULT_SSH_KEY_PATH = "~/.ssh/id_ed25519"
DEFAULT_SSH_CONFIG_PATH = "~/.ssh/config"
DEFAULT_K3S_API_PORT = 6443
DEFAULT_PORT_RANGE_START = 16443
DEFAULT_PORT_RANGE_SIZE = 10000
NUMERIC_CONFIG_KEYS = ("k3s_api_port", "port_range_start", "port_range_size")
ENV_VAR_MAPPING = {
    "remote_k3s_config_path": "REMOTE_K3S_CONFIG_PATH",
    "ssh_key_path": "SSH_KEY_PATH",
    "k3s_api_port": "K3S_API_PORT",
    "port_range_start": "PORT_RANGE_START",
    "port_range_size": "PORT_RANGE_SIZE",
    "inventory_path": "INVENTORY_PATH",
}


def _normalize_numeric_fields(config: Dict[str, Any]) -> Dict[str, Any]:
    normalized = dict(config)
    for key in NUMERIC_CONFIG_KEYS:
        if key not in normalized:
            continue
        try:
            normalized[key] = int(normalized[key])
        except (ValueError, TypeError):
            continue
    return normalized


def _get_canonical_int(config: Dict[str, Any], key: str, default: int) -> int:
    value = get_config_value(config, key, default)
    try:
        return int(value)
    except (ValueError, TypeError):
        return default


def load_config(config_path: str) -> Dict[str, Any]:
    """
    Load configuration from YAML file.

    Environment variables override file values. Variable names follow pattern:
    YAML key 'remote_k3s_config_path' → env var 'REMOTE_K3S_CONFIG_PATH'

    Args:
        config_path: Path to YAML config file

    Returns:
        dict: Merged configuration (file + env vars)
    """
    config: Dict[str, Any] = {}

    # Load from file if exists
    config_file = Path(config_path)
    if config_file.exists():
        try:
            with open(config_file) as f:
                file_config = _normalize_numeric_fields(yaml.safe_load(f) or {})
                config.update(file_config)
        except (yaml.YAMLError, OSError, IOError) as e:
            logger.warning(f"Failed to load config from {config_path}: {e}")

    # Override with environment variables
    for config_key, env_var in ENV_VAR_MAPPING.items():
        if env_var in os.environ:
            value = os.environ[env_var]
            # Safely convert to int with proper validation
            try:
                config[config_key] = int(value)
            except ValueError:
                # Keep as string if conversion fails
                config[config_key] = value

    return config


def get_config_value(config: Dict[str, Any], key: str, default: Any = None) -> Any:
    """
    Get value from config dict with optional default.

    Args:
        config: Configuration dictionary
        key: Key to retrieve
        default: Default value if key missing

    Returns:
        Config value or default
    """
    return config.get(key, default)


def resolve_inventory_path(config: Dict[str, Any], project_dir: Path) -> Path:
    """
    Resolve the inventory directory for this project.

    Resolution order:
    1. Existing path from config/env
    2. ./inventory inside the tool project
    3. First ancestor containing ansible/inventory
    4. Configured path even if missing
    5. ./inventory inside the tool project

    Args:
        config: Loaded configuration dictionary
        project_dir: Directory of the current tool/script

    Returns:
        Path to the best inventory directory candidate
    """
    configured_path: Optional[Path] = None
    inventory_from_config = get_config_value(config, 'inventory_path', None)
    if inventory_from_config:
        configured_path = Path(os.path.expanduser(str(inventory_from_config)))
        if configured_path.exists():
            return configured_path

    local_inventory = project_dir / "inventory"
    if local_inventory.exists():
        return local_inventory

    for parent in [project_dir, *project_dir.parents]:
        ancestor_inventory = parent / "ansible" / "inventory"
        if ancestor_inventory.exists():
            return ancestor_inventory

    if configured_path is not None:
        return configured_path

    return local_inventory


def load_effective_config(
    project_dir: Path,
    config_path: str | Path | None = None,
) -> EffectiveConfig:
    """
    Build the canonical typed configuration for the current project.

    Args:
        project_dir: Directory of the current tool/script
        config_path: Optional config file path. Defaults to the canonical user-data
            config path with legacy fallback.

    Returns:
        EffectiveConfig with env-over-file precedence preserved via load_config().
    """
    if config_path is not None:
        resolved_config_path = Path(config_path)
    else:
        resolved_config_path = get_config_file_path()
        for candidate in get_default_config_candidates(project_dir):
            if candidate.exists():
                resolved_config_path = candidate
                break

    config = load_config(os.path.expanduser(str(resolved_config_path)))

    return EffectiveConfig(
        inventory_path=resolve_inventory_path(config, project_dir),
        ssh_config_path=os.path.expanduser(DEFAULT_SSH_CONFIG_PATH),
        ssh_key_path=os.path.expanduser(
            str(get_config_value(config, "ssh_key_path", DEFAULT_SSH_KEY_PATH))
        ),
        remote_k3s_config_path=str(
            get_config_value(
                config,
                "remote_k3s_config_path",
                DEFAULT_REMOTE_K3S_CONFIG_PATH,
            )
        ),
        k3s_api_port=_get_canonical_int(
            config,
            "k3s_api_port",
            DEFAULT_K3S_API_PORT,
        ),
        port_range_start=_get_canonical_int(
            config,
            "port_range_start",
            DEFAULT_PORT_RANGE_START,
        ),
        port_range_size=_get_canonical_int(
            config,
            "port_range_size",
            DEFAULT_PORT_RANGE_SIZE,
        ),
    )
