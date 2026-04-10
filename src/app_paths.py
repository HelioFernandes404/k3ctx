"""Canonical filesystem paths for local application data."""

from __future__ import annotations

import os
from pathlib import Path

APP_NAME = "k3s-context-tunnel-manager"
YAML_DIRNAME = "yaml"
CONFIG_DIRNAME = "config"
KUBECONFIG_DIRNAME = "kubeconfigs"
LEGACY_CONFIG_DIRNAME = ".k9s-config"


def get_data_home() -> Path:
    override = os.getenv("XDG_DATA_HOME")
    if override:
        return Path(override).expanduser()
    return Path.home() / ".local" / "share"


def get_app_data_dir() -> Path:
    return get_data_home() / APP_NAME


def get_yaml_storage_dir() -> Path:
    return get_app_data_dir() / YAML_DIRNAME


def get_user_data_config_dir() -> Path:
    return get_yaml_storage_dir() / CONFIG_DIRNAME


def get_user_data_config_file_path() -> Path:
    return get_user_data_config_dir() / "config.yaml"


def get_config_dir() -> Path:
    override = os.getenv("K9S_CONFIG_DIR")
    if override:
        return Path(override).expanduser()
    return get_user_data_config_dir()


def get_config_file_path() -> Path:
    return get_config_dir() / "config.yaml"


def get_kubeconfig_cache_dir() -> Path:
    return get_yaml_storage_dir() / KUBECONFIG_DIRNAME


def get_kubeconfig_cache_path(context_name: str) -> Path:
    return get_kubeconfig_cache_dir() / f"{context_name}.yml"


def get_legacy_home_config_file_path() -> Path:
    return Path.home() / LEGACY_CONFIG_DIRNAME / "config.yaml"


def get_default_config_candidates(project_dir: Path) -> tuple[Path, ...]:
    candidates = (
        get_config_file_path(),
        get_user_data_config_file_path(),
        project_dir / "config.yaml",
        get_legacy_home_config_file_path(),
    )
    ordered: list[Path] = []
    for candidate in candidates:
        if candidate not in ordered:
            ordered.append(candidate)
    return tuple(ordered)


def get_legacy_project_root_yaml_files(project_dir: Path) -> tuple[Path, ...]:
    candidates: list[Path] = []
    for pattern in ("*.yml", "*.yaml"):
        for path in sorted(project_dir.glob(pattern)):
            if path.name == "config.yaml" or not path.is_file():
                continue
            candidates.append(path)
    return tuple(candidates)
