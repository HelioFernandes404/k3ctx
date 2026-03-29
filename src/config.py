"""Configuration management for k9s-config.

Supports loading config from YAML files and merging with environment variables.
Environment variables take precedence over file values.
Numeric values (ports, ranges) are normalized to int type.
"""

import os
import yaml
from pathlib import Path
from typing import Dict, Any, Optional
from .logging_config import get_logger

logger = get_logger()


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
                file_config = yaml.safe_load(f) or {}
                # Normalize numeric fields from YAML to int type
                for key in ['k3s_api_port', 'port_range_start', 'port_range_size']:
                    if key in file_config:
                        try:
                            file_config[key] = int(file_config[key])
                        except (ValueError, TypeError):
                            pass  # Keep as-is if conversion fails
                config.update(file_config)
        except (yaml.YAMLError, OSError, IOError) as e:
            logger.warning(f"Failed to load config from {config_path}: {e}")

    # Override with environment variables
    env_var_mapping = {
        'remote_k3s_config_path': 'REMOTE_K3S_CONFIG_PATH',
        'ssh_key_path': 'SSH_KEY_PATH',
        'k3s_api_port': 'K3S_API_PORT',
        'port_range_start': 'PORT_RANGE_START',
        'port_range_size': 'PORT_RANGE_SIZE',
        'inventory_path': 'INVENTORY_PATH',
    }

    for config_key, env_var in env_var_mapping.items():
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
