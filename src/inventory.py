"""
Inventory management for k9s-config.

Handles loading and parsing Ansible-style inventory files with support
for custom YAML tags like !vault.
"""

import sys
import subprocess
import yaml
from pathlib import Path
from typing import Dict, Any, Optional, Tuple

from .logging_config import get_logger


def update_inventory_repo(inventory_path: Path) -> Tuple[bool, str]:
    """
    Update the git repository containing the inventory files.

    Finds the git root of the inventory path and runs git pull.

    Args:
        inventory_path: Path to inventory directory

    Returns:
        tuple: (success: bool, message: str)
    """
    logger = get_logger()

    # Find git root from inventory path
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            cwd=inventory_path,
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode != 0:
            return False, "Inventory path is not in a git repository"

        git_root = Path(result.stdout.strip())
        logger.debug(f"Found git root: {git_root}")

    except subprocess.TimeoutExpired:
        return False, "Timeout finding git root"
    except FileNotFoundError:
        return False, "git command not found"

    # Run git pull
    try:
        logger.info(f"Updating inventory repository: {git_root}")
        result = subprocess.run(
            ["git", "pull", "--ff-only"],
            cwd=git_root,
            capture_output=True,
            text=True,
            timeout=30
        )

        if result.returncode == 0:
            output = result.stdout.strip()
            if "Already up to date" in output:
                logger.debug("Inventory already up to date")
                return True, "Already up to date"
            else:
                logger.info(f"Inventory updated: {output}")
                return True, f"Updated: {output.split(chr(10))[0]}"
        else:
            error = result.stderr.strip() or result.stdout.strip()
            logger.warning(f"Git pull failed: {error}")
            return False, f"Git pull failed: {error}"

    except subprocess.TimeoutExpired:
        return False, "Timeout during git pull"
    except Exception as e:
        return False, f"Error during git pull: {e}"


def load_inventories(inventory_path: Optional[Path] = None) -> Dict[str, Any]:
    """
    Load all *_hosts.yml from inventory/ directory.

    Args:
        inventory_path: Path to inventory directory (default: ./inventory)

    Returns:
        dict: {company_name: inventory_data}
    """
    if inventory_path is None:
        inventory_path = Path(__file__).parent.parent / "inventory"

    inventories: Dict[str, Any] = {}
    if not inventory_path.exists():
        return inventories

    # Custom YAML loader that ignores unknown tags (like !vault)
    class VaultIgnoreLoader(yaml.SafeLoader):
        pass

    def ignore_unknown_tag(loader: Any, tag_suffix: Any, node: Any) -> Any:
        if isinstance(node, yaml.MappingNode):
            return loader.construct_mapping(node)
        elif isinstance(node, yaml.SequenceNode):
            return loader.construct_sequence(node)
        else:
            return node.value

    VaultIgnoreLoader.add_multi_constructor('', ignore_unknown_tag)

    for inv_file in sorted(inventory_path.glob("*_hosts.yml")):
        company = inv_file.stem.replace("_hosts", "")
        try:
            with open(inv_file) as f:
                data = yaml.load(f, Loader=VaultIgnoreLoader)
                inventories[company] = data
        except Exception as e:
            print(f"Warning: Failed to load {inv_file}: {e}", file=sys.stderr)

    return inventories


def extract_hosts_from_inventory(inv_data: Any) -> Dict[str, Dict[str, Any]]:
    """
    Extract all hosts from an inventory structure.

    Args:
        inv_data: Parsed inventory YAML data

    Returns:
        dict: {host_name: {"group": str, "config": dict}}
    """
    hosts: Dict[str, Dict[str, Any]] = {}
    if not isinstance(inv_data, dict) or "all" not in inv_data:
        return hosts

    all_data = inv_data["all"]
    if "children" not in all_data:
        return hosts

    for group_name, group_data in all_data["children"].items():
        if isinstance(group_data, dict) and "hosts" in group_data:
            # Check if hosts is not None and is a dict
            hosts_data = group_data["hosts"]
            if hosts_data and isinstance(hosts_data, dict):
                for host_name, host_config in hosts_data.items():
                    hosts[host_name] = {
                        "group": group_name,
                        "config": host_config or {}
                    }

    return hosts
