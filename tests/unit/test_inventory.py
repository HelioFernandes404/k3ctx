"""Unit tests for inventory module."""

from types import SimpleNamespace
import tempfile
from pathlib import Path
from typing import Any

import pytest
import yaml
from _pytest.capture import CaptureFixture
from src.inventory import extract_hosts_from_inventory, load_inventories, update_inventory_repo


class TestLoadInventories:
    """Tests for load_inventories function."""

    def test_loads_valid_inventory_file(self) -> None:
        """Loads a valid inventory YAML file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)
            inv_file = inv_dir / "test_hosts.yml"

            inv_data: dict[str, Any] = {
                "all": {
                    "children": {
                        "k3s_cluster": {
                            "hosts": {
                                "testhost": {"ansible_host": "1.2.3.4"}
                            }
                        }
                    }
                }
            }

            with open(inv_file, 'w') as f:
                yaml.dump(inv_data, f)

            result = load_inventories(inv_dir)

            assert "test" in result
            assert result["test"]["all"]["children"]["k3s_cluster"]["hosts"]["testhost"]["ansible_host"] == "1.2.3.4"

    def test_ignores_vault_tags_in_yaml(self) -> None:
        """Loads YAML with !vault tags without crashing."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)
            inv_file = inv_dir / "company_hosts.yml"

            yaml_content = """
all:
  vars:
    password: !vault |
      $ANSIBLE_VAULT;1.1;AES256
      secret
  children:
    k3s_cluster:
      hosts:
        testhost:
          ansible_host: 1.2.3.4
"""
            with open(inv_file, 'w') as f:
                f.write(yaml_content)

            result = load_inventories(inv_dir)

            assert "company" in result
            assert "k3s_cluster" in result["company"]["all"]["children"]

    def test_returns_empty_dict_for_nonexistent_directory(self) -> None:
        """Returns empty dict when inventory directory doesn't exist."""
        result = load_inventories(Path("/nonexistent/path"))
        assert result == {}

    def test_loads_multiple_inventory_files(self) -> None:
        """Loads multiple *_hosts.yml files."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)

            # Create two inventory files
            for company in ["company1", "company2"]:
                inv_file = inv_dir / f"{company}_hosts.yml"
                inv_data: dict[str, Any] = {"all": {"children": {"k3s_cluster": {"hosts": {}}}}}
                with open(inv_file, 'w') as f:
                    yaml.dump(inv_data, f)

            result = load_inventories(inv_dir)

            assert "company1" in result
            assert "company2" in result

    def test_skips_malformed_yaml_files(self, capsys: CaptureFixture[str]) -> None:
        """Skips files with invalid YAML syntax."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)

            # Create valid file
            valid_file = inv_dir / "valid_hosts.yml"
            with open(valid_file, 'w') as f:
                yaml.dump({"all": {}}, f)

            # Create invalid file
            invalid_file = inv_dir / "invalid_hosts.yml"
            with open(invalid_file, 'w') as f:
                f.write("invalid: yaml: syntax: [")

            result = load_inventories(inv_dir)

            # Should load valid file, skip invalid
            assert "valid" in result
            assert "invalid" not in result


class TestExtractHostsFromInventory:
    """Tests for extract_hosts_from_inventory function."""

    def test_extracts_hosts_from_valid_inventory(self) -> None:
        """Extracts hosts with group, config, and group vars."""
        inv_data: dict[str, Any] = {
            "all": {
                "vars": {
                    "company": "acme"
                },
                "children": {
                    "k3s_cluster": {
                        "vars": {
                            "gateway": "bastion.example"
                        },
                        "hosts": {
                            "host1": {"ansible_host": "1.2.3.4"},
                            "host2": {"ansible_host": "5.6.7.8"}
                        }
                    }
                }
            }
        }

        hosts = extract_hosts_from_inventory(inv_data)

        assert len(hosts) == 2
        assert "host1" in hosts
        assert hosts["host1"]["group"] == "k3s_cluster"
        assert hosts["host1"]["config"]["ansible_host"] == "1.2.3.4"
        assert hosts["host1"]["group_vars"]["company"] == "acme"
        assert hosts["host1"]["group_vars"]["gateway"] == "bastion.example"

    def test_extracts_hosts_from_multiple_groups(self) -> None:
        """Extracts hosts from multiple groups."""
        inv_data: dict[str, Any] = {
            "all": {
                "children": {
                    "k3s_cluster": {
                        "hosts": {"host1": {}}
                    },
                    "monitoring": {
                        "hosts": {"host2": {}}
                    }
                }
            }
        }

        hosts = extract_hosts_from_inventory(inv_data)

        assert len(hosts) == 2
        assert hosts["host1"]["group"] == "k3s_cluster"
        assert hosts["host2"]["group"] == "monitoring"

    def test_handles_none_host_config(self) -> None:
        """Handles None host config as empty dict."""
        inv_data: dict[str, Any] = {
            "all": {
                "children": {
                    "k3s_cluster": {
                        "hosts": {"host1": None}
                    }
                }
            }
        }

        hosts = extract_hosts_from_inventory(inv_data)

        assert hosts["host1"]["config"] == {}
        assert hosts["host1"]["group_vars"] == {}

    def test_extracts_group_vars_from_nested_group(self) -> None:
        """Merges inherited vars for nested groups without breaking host config."""
        inv_data: dict[str, Any] = {
            "all": {
                "vars": {
                    "company": "acme"
                },
                "children": {
                    "platform": {
                        "vars": {
                            "region": "us-east-1"
                        },
                        "children": {
                            "k3s_cluster": {
                                "vars": {
                                    "gateway": "bastion.example"
                                },
                                "hosts": {
                                    "host1": {"ansible_host": "1.2.3.4"}
                                }
                            }
                        }
                    }
                }
            }
        }

        hosts = extract_hosts_from_inventory(inv_data)

        assert hosts["host1"]["group"] == "k3s_cluster"
        assert hosts["host1"]["group_vars"] == {
            "company": "acme",
            "region": "us-east-1",
            "gateway": "bastion.example",
        }

    def test_returns_empty_for_invalid_inventory(self) -> None:
        """Returns empty dict for malformed inventory."""
        assert extract_hosts_from_inventory({}) == {}
        assert extract_hosts_from_inventory({"all": {}}) == {}
        assert extract_hosts_from_inventory(None) == {}

    def test_returns_empty_when_no_hosts_section(self) -> None:
        """Returns empty dict when group has no hosts."""
        inv_data: dict[str, Any] = {
            "all": {
                "children": {
                    "k3s_cluster": {}
                }
            }
        }

        hosts = extract_hosts_from_inventory(inv_data)

        assert hosts == {}


class TestUpdateInventoryRepo:
    """Tests for git-backed inventory refresh behavior."""

    def test_skips_refresh_when_git_repository_is_dirty(
        self,
        monkeypatch: pytest.MonkeyPatch,
        tmp_path: Path,
    ) -> None:
        calls: list[list[str]] = []

        def fake_run(
            cmd: list[str],
            cwd: Path,
            capture_output: bool,
            text: bool,
            timeout: int,
        ) -> SimpleNamespace:
            calls.append(cmd)
            if cmd[:3] == ["git", "rev-parse", "--show-toplevel"]:
                return SimpleNamespace(returncode=0, stdout=str(tmp_path), stderr="")
            if cmd[:3] == ["git", "status", "--porcelain"]:
                return SimpleNamespace(returncode=0, stdout=" M inventory/acme_hosts.yml\n", stderr="")
            raise AssertionError(f"Unexpected command: {cmd}")

        monkeypatch.setattr("src.inventory.subprocess.run", fake_run)

        success, message = update_inventory_repo(tmp_path / "inventory")

        assert success is False
        assert "local changes" in message
        assert ["git", "pull", "--ff-only"] not in calls
