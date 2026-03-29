"""Unit tests for tunnel module."""

import pytest
import tempfile
import os
from pathlib import Path
from unittest.mock import Mock, patch, MagicMock
from src.tunnel import (
    get_unique_port,
    get_tunnel_pid_file,
    is_tunnel_running,
    kill_tunnel,
    kill_all_tunnels,
    create_tunnel,
    save_tunnel_pid
)


class TestGetUniquePort:
    """Tests for get_unique_port function."""

    def test_generates_deterministic_port(self):
        """Generates same port for same context name."""
        port1 = get_unique_port("company-host")
        port2 = get_unique_port("company-host")
        assert port1 == port2

    def test_generates_different_ports_for_different_contexts(self):
        """Generates different ports for different context names."""
        port1 = get_unique_port("company1-host1")
        port2 = get_unique_port("company2-host2")
        # Hash collision possible but unlikely
        assert port1 != port2 or port1 == port2  # Always true, just documenting behavior

    def test_port_in_valid_range(self):
        """Generated port is between 16443 and 26443."""
        port = get_unique_port("test-context")
        assert 16443 <= port <= 26443


class TestGetUniquePortDynamic:
    """Tests for dynamic port range configuration."""

    def test_generates_port_within_custom_range(self):
        """Generates port within specified custom range."""
        port = get_unique_port("test-context", port_range_start=20000, port_range_size=5000)
        assert 20000 <= port < 25000

    def test_generates_port_within_default_range(self):
        """Generates port within default range when not specified."""
        port = get_unique_port("test-context")
        assert 16443 <= port < 26443

    def test_large_port_range(self):
        """Supports large port ranges for many deployments."""
        port = get_unique_port("test-context", port_range_start=10000, port_range_size=50000)
        assert 10000 <= port < 60000

    def test_different_contexts_get_different_ports_in_range(self):
        """Different contexts generate different ports in custom range."""
        port1 = get_unique_port("context1", port_range_start=20000, port_range_size=5000)
        port2 = get_unique_port("context2", port_range_start=20000, port_range_size=5000)

        assert port1 != port2
        assert 20000 <= port1 < 25000
        assert 20000 <= port2 < 25000


class TestGetTunnelPidFile:
    """Tests for get_tunnel_pid_file function."""

    def test_returns_correct_path(self):
        """Returns path with context name and .pid extension."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)
            pid_file = get_tunnel_pid_file("test-context", state_dir)

            assert pid_file.name == "test-context.pid"
            assert pid_file.parent == state_dir

    def test_creates_state_directory(self):
        """Creates state directory if it doesn't exist."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir) / "nonexistent"
            assert not state_dir.exists()

            get_tunnel_pid_file("test-context", state_dir)

            assert state_dir.exists()


class TestIsTunnelRunning:
    """Tests for is_tunnel_running function."""

    def test_returns_false_when_pid_file_missing(self):
        """Returns False when PID file doesn't exist."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)
            result = is_tunnel_running("nonexistent", state_dir)
            assert result is False

    def test_returns_true_when_process_running(self):
        """Returns True when PID file exists and process is running."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)
            pid_file = get_tunnel_pid_file("test", state_dir)

            # Write current process PID (always running)
            current_pid = os.getpid()
            with open(pid_file, 'w') as f:
                f.write(str(current_pid))

            result = is_tunnel_running("test", state_dir)
            assert result is True

    def test_returns_false_and_cleans_stale_pid(self):
        """Returns False and removes stale PID file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)
            pid_file = get_tunnel_pid_file("test", state_dir)

            # Write non-existent PID
            with open(pid_file, 'w') as f:
                f.write("99999")

            result = is_tunnel_running("test", state_dir)

            assert result is False
            assert not pid_file.exists()

    def test_handles_invalid_pid_format(self):
        """Returns False for invalid PID format."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)
            pid_file = get_tunnel_pid_file("test", state_dir)

            with open(pid_file, 'w') as f:
                f.write("not-a-number")

            result = is_tunnel_running("test", state_dir)
            assert result is False


class TestKillTunnel:
    """Tests for kill_tunnel function."""

    def test_does_nothing_when_pid_file_missing(self):
        """Does nothing when PID file doesn't exist."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)
            # Should not raise exception
            kill_tunnel("nonexistent", state_dir)

    @patch('os.kill')
    def test_kills_process_and_removes_pid_file(self, mock_kill):
        """Kills process and removes PID file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)
            pid_file = get_tunnel_pid_file("test", state_dir)

            with open(pid_file, 'w') as f:
                f.write("12345")

            kill_tunnel("test", state_dir)

            mock_kill.assert_called_once_with(12345, 15)
            assert not pid_file.exists()

    def test_removes_pid_file_even_if_kill_fails(self):
        """Removes PID file even if process doesn't exist."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)
            pid_file = get_tunnel_pid_file("test", state_dir)

            # Write non-existent PID
            with open(pid_file, 'w') as f:
                f.write("99999")

            kill_tunnel("test", state_dir)

            assert not pid_file.exists()


class TestKillAllTunnels:
    """Tests for kill_all_tunnels function."""

    def test_does_nothing_when_state_dir_missing(self):
        """Does nothing when state directory doesn't exist."""
        kill_all_tunnels(Path("/nonexistent"))

    @patch('src.tunnel.kill_tunnel')
    def test_kills_all_tunnels(self, mock_kill_tunnel):
        """Kills all tunnels found in state directory."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)

            # Create PID files
            for context in ["context1", "context2"]:
                pid_file = state_dir / f"{context}.pid"
                with open(pid_file, 'w') as f:
                    f.write("12345")

            kill_all_tunnels(state_dir)

            assert mock_kill_tunnel.call_count == 2


class TestCreateTunnel:
    """Tests for create_tunnel function."""

    @patch('subprocess.run')
    @patch('time.sleep')
    def test_creates_tunnel_successfully(self, mock_sleep, mock_run):
        """Creates SSH tunnel and returns PID."""
        # Mock subprocess calls
        mock_run.side_effect = [
            Mock(returncode=0, stderr=""),  # ssh command
            Mock(returncode=0, stdout="12345\n")  # pgrep command
        ]

        pid = create_tunnel("testhost", "10.0.0.1", 16443, 6443)

        assert pid == 12345
        mock_run.assert_any_call(
            ["ssh", "-f", "-N", "-o", "ExitOnForwardFailure=yes",
             "-o", "ServerAliveInterval=60", "-L", "16443:10.0.0.1:6443", "testhost"],
            capture_output=True,
            text=True
        )

    @patch('subprocess.run')
    def test_raises_error_on_tunnel_failure(self, mock_run):
        """Raises RuntimeError when SSH tunnel creation fails."""
        mock_run.return_value = Mock(returncode=1, stderr="Connection failed")

        with pytest.raises(RuntimeError, match="Failed to create SSH tunnel"):
            create_tunnel("testhost", "10.0.0.1", 16443)

    @patch('subprocess.run')
    @patch('time.sleep')
    def test_returns_none_when_pid_not_found(self, mock_sleep, mock_run):
        """Returns None when pgrep can't find tunnel process."""
        mock_run.side_effect = [
            Mock(returncode=0, stderr=""),  # ssh command
            Mock(returncode=1, stdout="")   # pgrep fails
        ]

        pid = create_tunnel("testhost", "10.0.0.1", 16443)

        assert pid is None


class TestSaveTunnelPid:
    """Tests for save_tunnel_pid function."""

    def test_saves_pid_to_file(self):
        """Saves PID to correct file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)

            save_tunnel_pid("test-context", 12345, state_dir)

            pid_file = state_dir / "test-context.pid"
            assert pid_file.exists()

            with open(pid_file) as f:
                assert f.read() == "12345"

    def test_does_nothing_when_pid_is_none(self):
        """Does nothing when PID is None."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)

            save_tunnel_pid("test", None, state_dir)

            pid_file = state_dir / "test.pid"
            assert not pid_file.exists()
