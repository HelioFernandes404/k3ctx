"""
Kubeconfig management for k9s-config.

Handles YAML manipulation, server URL updates, and merging contexts
into ~/.kube/config.
"""

import yaml
from io import StringIO
from pathlib import Path
import shutil
from typing import Optional
from .logging_config import get_logger

logger = get_logger()


def update_kubeconfig_server(
    yaml_text: str,
    ip: str,
    port: int,
    use_localhost: bool = False,
    local_port: Optional[int] = None
) -> str:
    """
    Update server URL in kubeconfig YAML.

    Args:
        yaml_text: Kubeconfig YAML as string
        ip: Internal IP address
        port: Remote K3s port (usually 6443)
        use_localhost: If True, use localhost with local_port
        local_port: Local tunnel port (when use_localhost=True)

    Returns:
        str: Updated kubeconfig YAML

    Raises:
        RuntimeError: If kubeconfig structure is invalid
    """
    data = yaml.safe_load(StringIO(yaml_text))
    if not isinstance(data, dict) or "clusters" not in data:
        raise RuntimeError(
            "Fetched file doesn't look like a kubeconfig (no 'clusters' key)."
        )

    # modify the first cluster's server
    try:
        if use_localhost and local_port:
            data["clusters"][0]["cluster"]["server"] = f"https://127.0.0.1:{local_port}"
        else:
            data["clusters"][0]["cluster"]["server"] = f"https://{ip}:{port}"
    except Exception as e:
        raise RuntimeError(f"Failed updating server field in kubeconfig structure: {e}")

    outbuf = StringIO()
    yaml.safe_dump(data, outbuf, default_flow_style=False)
    return outbuf.getvalue()


def merge_kubeconfig(new_config_text: str, context_name: str) -> Path:
    """
    Merge new kubeconfig into ~/.kube/config.

    Creates backup of existing config, merges new context, and sets it as current.

    Args:
        new_config_text: New kubeconfig YAML as string
        context_name: Name for the new context (e.g., "company-host")

    Returns:
        Path: Path to updated kubeconfig file

    Side effects:
        - Backs up existing ~/.kube/config to ~/.kube/config.bak
        - Creates ~/.kube directory if missing
        - Sets new context as current-context
    """
    kubeconfig_path = Path.home() / ".kube" / "config"

    # Ensure ~/.kube directory exists
    kubeconfig_path.parent.mkdir(parents=True, exist_ok=True)

    # Load new config
    new_config = yaml.safe_load(StringIO(new_config_text))

    # Load existing config or create empty structure
    if kubeconfig_path.exists():
        with open(kubeconfig_path) as f:
            existing_config = yaml.safe_load(f) or {}
    else:
        existing_config = {}

    # Ensure required keys exist
    for key in ['clusters', 'contexts', 'users']:
        if key not in existing_config:
            existing_config[key] = []

    # Rename cluster, user, and context to use context_name
    if new_config.get('clusters'):
        cluster = new_config['clusters'][0]
        cluster['name'] = context_name

    if new_config.get('users'):
        user = new_config['users'][0]
        user['name'] = context_name

    if new_config.get('contexts'):
        context = new_config['contexts'][0]
        context['name'] = context_name
        context['context']['cluster'] = context_name
        context['context']['user'] = context_name

    # Remove existing entries with same name
    for key in ['clusters', 'contexts', 'users']:
        existing_config[key] = [
            item for item in existing_config[key]
            if item.get('name') != context_name
        ]

    # Add new entries
    for key in ['clusters', 'contexts', 'users']:
        if new_config.get(key):
            existing_config[key].extend(new_config[key])

    # Set as current context
    existing_config['current-context'] = context_name

    # Backup existing config
    if kubeconfig_path.exists():
        backup_path = kubeconfig_path.with_suffix('.bak')
        shutil.copy2(kubeconfig_path, backup_path)
        logger.info(f"Backed up existing kubeconfig: {backup_path}")

    # Write merged config
    with open(kubeconfig_path, 'w') as f:
        yaml.safe_dump(existing_config, f, default_flow_style=False)

    return kubeconfig_path
