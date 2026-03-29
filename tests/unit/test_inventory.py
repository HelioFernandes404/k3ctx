"""Unit tests for inventory module."""

import pytest
import tempfile
import yaml
from pathlib import Path
from src.inventory import load_inventories, extract_hosts_from_inventory


class TestLoadInventories:
    """Tests for load_inventories function."""

    def test_loads_valid_inventory_file(self):
        """Loads a valid inventory YAML file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)
            inv_file = inv_dir / "test_hosts.yml"

            inv_data = {
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

    def test_ignores_vault_tags_in_yaml(self):
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

    def test_returns_empty_dict_for_nonexistent_directory(self):
        """Returns empty dict when inventory directory doesn't exist."""
        result = load_inventories(Path("/nonexistent/path"))
        assert result == {}

    def test_loads_multiple_inventory_files(self):
        """Loads multiple *_hosts.yml files."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)

            # Create two inventory files
            for company in ["company1", "company2"]:
                inv_file = inv_dir / f"{company}_hosts.yml"
                inv_data = {"all": {"children": {"k3s_cluster": {"hosts": {}}}}}
                with open(inv_file, 'w') as f:
                    yaml.dump(inv_data, f)

            result = load_inventories(inv_dir)

            assert "company1" in result
            assert "company2" in result

    def test_skips_malformed_yaml_files(self, capsys):
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

    def test_extracts_hosts_from_valid_inventory(self):
        """Extracts hosts with group and config."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {
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

    def test_extracts_hosts_from_multiple_groups(self):
        """Extracts hosts from multiple groups."""
        inv_data = {
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

    def test_handles_none_host_config(self):
        """Handles None host config as empty dict."""
        inv_data = {
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

    def test_returns_empty_for_invalid_inventory(self):
        """Returns empty dict for malformed inventory."""
        assert extract_hosts_from_inventory({}) == {}
        assert extract_hosts_from_inventory({"all": {}}) == {}
        assert extract_hosts_from_inventory(None) == {}

    def test_returns_empty_when_no_hosts_section(self):
        """Returns empty dict when group has no hosts."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {}
                }
            }
        }

        hosts = extract_hosts_from_inventory(inv_data)

        assert hosts == {}
