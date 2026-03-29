"""Unit tests for SSH module."""

import pytest
import tempfile
from pathlib import Path
from unittest.mock import Mock, MagicMock, patch
from src.ssh import load_ssh_config, choose_first, get_internal_ip


class TestLoadSshConfig:
    """Tests for load_ssh_config function."""

    def test_returns_empty_dict_when_config_missing(self):
        """Returns empty dict when SSH config doesn't exist."""
        result = load_ssh_config("testhost", "/nonexistent/ssh/config")
        assert result == {}

    def test_parses_ssh_config_file(self):
        """Parses SSH config and returns host configuration."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='_ssh_config', delete=False) as f:
            f.write("""
Host testhost
    HostName 192.168.1.100
    User ubuntu
    Port 2222
    IdentityFile ~/.ssh/test_key
""")
            config_path = f.name

        try:
            result = load_ssh_config("testhost", config_path)

            assert result["hostname"] == "192.168.1.100"
            assert result["user"] == "ubuntu"
            assert result["port"] == "2222"
        finally:
            Path(config_path).unlink()

    def test_returns_default_ssh_config_for_unknown_host(self):
        """Returns default config for host not in SSH config."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='_ssh_config', delete=False) as f:
            f.write("""
Host knownhost
    HostName 1.2.3.4
""")
            config_path = f.name

        try:
            result = load_ssh_config("unknownhost", config_path)

            # Should return with hostname matching the alias
            assert "hostname" in result
        finally:
            Path(config_path).unlink()


class TestChooseFirst:
    """Tests for choose_first helper function."""

    def test_returns_first_element_of_list(self):
        """Returns first element from list."""
        assert choose_first(["first", "second", "third"]) == "first"

    def test_returns_first_element_of_tuple(self):
        """Returns first element from tuple."""
        assert choose_first(("first", "second")) == "first"

    def test_returns_value_if_not_list(self):
        """Returns value itself if not a list or tuple."""
        assert choose_first("single_value") == "single_value"

    def test_returns_default_for_empty_list(self):
        """Returns default value for empty list."""
        assert choose_first([], default="default") == "default"

    def test_returns_default_for_none(self):
        """Returns default value for None."""
        assert choose_first(None, default="default") == "default"


class TestGetInternalIp:
    """Tests for get_internal_ip function."""

    def test_returns_first_valid_ipv4(self):
        """Returns first non-loopback IPv4 address found."""
        mock_ssh = MagicMock()

        # Mock command execution
        mock_stdout = MagicMock()
        mock_stdout.read.return_value = b"10.0.0.100\n"

        mock_ssh.exec_command.return_value = (None, mock_stdout, None)

        result = get_internal_ip(mock_ssh)

        assert result == "10.0.0.100"

    def test_tries_multiple_commands_until_success(self):
        """Tries multiple detection commands until one succeeds."""
        mock_ssh = MagicMock()

        # First command returns empty, second returns IP
        mock_stdout1 = MagicMock()
        mock_stdout1.read.return_value = b""

        mock_stdout2 = MagicMock()
        mock_stdout2.read.return_value = b"192.168.1.50\n"

        mock_ssh.exec_command.side_effect = [
            (None, mock_stdout1, None),
            (None, mock_stdout2, None)
        ]

        result = get_internal_ip(mock_ssh)

        assert result == "192.168.1.50"
        assert mock_ssh.exec_command.call_count == 2

    def test_skips_loopback_addresses(self):
        """Skips loopback addresses (127.x.x.x)."""
        mock_ssh = MagicMock()

        mock_stdout1 = MagicMock()
        mock_stdout1.read.return_value = b"127.0.0.1\n"

        mock_stdout2 = MagicMock()
        mock_stdout2.read.return_value = b"10.0.0.1\n"

        mock_ssh.exec_command.side_effect = [
            (None, mock_stdout1, None),
            (None, mock_stdout2, None)
        ]

        result = get_internal_ip(mock_ssh)

        assert result == "10.0.0.1"

    def test_handles_multiple_ips_from_command(self):
        """Handles command returning multiple IPs (takes first)."""
        mock_ssh = MagicMock()

        mock_stdout = MagicMock()
        mock_stdout.read.return_value = b"10.0.0.1 192.168.1.1 172.16.0.1\n"

        mock_ssh.exec_command.return_value = (None, mock_stdout, None)

        result = get_internal_ip(mock_ssh)

        assert result == "10.0.0.1"

    def test_raises_error_when_no_ip_found(self):
        """Raises RuntimeError when no valid IP is detected."""
        mock_ssh = MagicMock()

        # All commands return empty
        mock_stdout = MagicMock()
        mock_stdout.read.return_value = b""

        mock_ssh.exec_command.return_value = (None, mock_stdout, None)

        with pytest.raises(RuntimeError, match="Could not detect internal IPv4"):
            get_internal_ip(mock_ssh)
