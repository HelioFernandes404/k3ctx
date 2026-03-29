"""Unit tests for CLI module."""

import pytest
import tempfile
import yaml
from pathlib import Path
from unittest.mock import patch, MagicMock
from src.cli import (
    NonInteractiveTerminalError,
    confirm_action,
    select_company,
    select_host,
)


class TestSelectCompany:
    """Tests for select_company function."""

    def test_prompts_user_and_returns_selection(self):
        """Prompts user to select company and returns choice."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)

            # Create inventory files
            for company in ["company1", "company2"]:
                inv_file = inv_dir / f"{company}_hosts.yml"
                with open(inv_file, 'w') as f:
                    yaml.dump({"all": {}}, f)

            # Mock questionary.autocomplete
            with patch('src.cli.sys.stdin.isatty', return_value=True), \
                 patch('src.cli.sys.stdout.isatty', return_value=True), \
                 patch('src.cli.questionary.autocomplete') as mock_autocomplete:
                mock_autocomplete.return_value.ask.return_value = "company1"
                company, inv_data = select_company(inv_dir)

            assert company == "company1"
            assert isinstance(inv_data, dict)

    def test_handles_cancellation(self):
        """Returns None when user cancels selection."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)

            inv_file = inv_dir / "test_hosts.yml"
            with open(inv_file, 'w') as f:
                yaml.dump({"all": {}}, f)

            # Mock user cancelling (ESC or Ctrl+C)
            with patch('src.cli.sys.stdin.isatty', return_value=True), \
                 patch('src.cli.sys.stdout.isatty', return_value=True), \
                 patch('src.cli.questionary.autocomplete') as mock_autocomplete:
                mock_autocomplete.return_value.ask.return_value = None
                company, inv_data = select_company(inv_dir)

            assert company is None
            assert inv_data is None

    def test_exits_when_no_inventories_found(self):
        """Exits with error when no inventory files exist."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)

            with pytest.raises(SystemExit) as exc_info:
                select_company(inv_dir)

            assert exc_info.value.code == 1

    def test_reports_the_inventory_path_when_none_are_found(self, capsys):
        """Includes the searched inventory path in the error output."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)

            with pytest.raises(SystemExit):
                select_company(inv_dir)

            captured = capsys.readouterr()
            assert str(inv_dir) in captured.err

    def test_fails_fast_without_interactive_terminal(self):
        """Raises clear error when no interactive terminal is available."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inv_dir = Path(tmpdir)

            inv_file = inv_dir / "test_hosts.yml"
            with open(inv_file, 'w') as f:
                yaml.dump({"all": {}}, f)

            with patch('src.cli.sys.stdin.isatty', return_value=False), \
                 patch('src.cli.sys.stdout.isatty', return_value=False):
                with pytest.raises(NonInteractiveTerminalError, match="interactive terminal"):
                    select_company(inv_dir)


class TestSelectHost:
    """Tests for select_host function."""

    def test_prompts_user_and_returns_host(self):
        """Prompts user to select host and returns choice."""
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

        with patch('src.cli.sys.stdin.isatty', return_value=True), \
             patch('src.cli.sys.stdout.isatty', return_value=True), \
             patch('src.cli.questionary.autocomplete') as mock_autocomplete:
            # Return the full label that would be displayed
            mock_autocomplete.return_value.ask.return_value = "host1 (k3s_cluster)"
            host_name, host_info = select_host("test", inv_data)

        assert host_name == "host1"
        assert host_info["group"] == "k3s_cluster"
        assert host_info["config"]["ansible_host"] == "1.2.3.4"

    def test_displays_vpn_indicator_when_required(self):
        """Displays [VPN] indicator for hosts requiring VPN."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {
                        "vars": {
                            "argocd_use_socks5_proxy": True
                        },
                        "hosts": {
                            "vpnhost": {"ansible_host": "192.168.1.100"}
                        }
                    }
                }
            }
        }

        with patch('src.cli.sys.stdin.isatty', return_value=True), \
             patch('src.cli.sys.stdout.isatty', return_value=True), \
             patch('src.cli.questionary.autocomplete') as mock_autocomplete:
            # 192.168.x.x will also trigger sshuttle, so include both indicators
            mock_autocomplete.return_value.ask.return_value = "vpnhost (k3s_cluster) [VPN] [sshuttle 192.168.1.0/24]"
            select_host("test", inv_data)

        # Verify that the choice label contains [VPN]
        call_args = mock_autocomplete.call_args
        choices = call_args[1]['choices']
        assert any("[VPN]" in choice for choice in choices)

    def test_displays_sshuttle_indicator_for_private_ip(self):
        """Displays [sshuttle] indicator for private IPs."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {
                        "hosts": {
                            "privatehost": {"ansible_host": "10.0.0.100"}
                        }
                    }
                }
            }
        }

        with patch('src.cli.sys.stdin.isatty', return_value=True), \
             patch('src.cli.sys.stdout.isatty', return_value=True), \
             patch('src.cli.questionary.autocomplete') as mock_autocomplete:
            mock_autocomplete.return_value.ask.return_value = "privatehost (k3s_cluster) [sshuttle 10.0.0.0/24]"
            select_host("test", inv_data)

        # Verify that the choice label contains [sshuttle]
        call_args = mock_autocomplete.call_args
        choices = call_args[1]['choices']
        assert any("[sshuttle" in choice for choice in choices)

    def test_handles_cancellation(self):
        """Returns None when user cancels selection."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {
                        "hosts": {
                            "testhost": {}
                        }
                    }
                }
            }
        }

        with patch('src.cli.sys.stdin.isatty', return_value=True), \
             patch('src.cli.sys.stdout.isatty', return_value=True), \
             patch('src.cli.questionary.autocomplete') as mock_autocomplete:
            mock_autocomplete.return_value.ask.return_value = None
            host_name, host_info = select_host("test", inv_data)

        assert host_name is None
        assert host_info is None

    def test_exits_when_no_hosts_found(self):
        """Exits with error when inventory has no hosts."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {}
                }
            }
        }

        with pytest.raises(SystemExit) as exc_info:
            select_host("test", inv_data)

        assert exc_info.value.code == 1

    def test_fails_fast_without_interactive_terminal(self):
        """Raises clear error when no interactive terminal is available."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {
                        "hosts": {
                            "testhost": {}
                        }
                    }
                }
            }
        }

        with patch('src.cli.sys.stdin.isatty', return_value=False), \
             patch('src.cli.sys.stdout.isatty', return_value=False):
            with pytest.raises(NonInteractiveTerminalError, match="interactive terminal"):
                select_host("test", inv_data)


class TestConfirmAction:
    """Tests for confirmation prompts."""

    def test_prompts_user_and_returns_confirmation(self):
        """Returns the confirmation value from questionary."""
        with patch('src.cli.sys.stdin.isatty', return_value=True), \
             patch('src.cli.sys.stdout.isatty', return_value=True), \
             patch('src.cli.questionary.confirm') as mock_confirm:
            mock_confirm.return_value.ask.return_value = True

            assert confirm_action("Continue?") is True

    def test_fails_fast_without_interactive_terminal(self):
        """Raises clear error when confirm prompt runs without TTY."""
        with patch('src.cli.sys.stdin.isatty', return_value=False), \
             patch('src.cli.sys.stdout.isatty', return_value=False):
            with pytest.raises(NonInteractiveTerminalError, match="interactive terminal"):
                confirm_action("Continue?")
