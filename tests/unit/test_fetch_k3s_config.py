"""Unit tests for fetch_k3s_config main script."""

import pytest
import tempfile
from unittest.mock import Mock, MagicMock, patch
from pathlib import Path
import sys

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from fetch_k3s_config import fetch_and_merge_kubeconfig
from src.kubeconfig import update_kubeconfig_server


class TestFetchAndMergeKubeconfig:
    """Tests for fetch_and_merge_kubeconfig function."""

    def test_returns_context_name_and_port(self):
        """Returns tuple of context_name, local_port, internal_ip, new_content, and was_cached."""
        mock_ssh = MagicMock()
        mock_ssh.exec_command.return_value = (
            MagicMock(),  # stdin
            MagicMock(read=lambda: b"10.0.0.1"),  # stdout
            MagicMock()   # stderr
        )

        # Mock fetch_remote_file_cached
        with patch('fetch_k3s_config.fetch_remote_file_cached') as mock_fetch:
            mock_fetch.return_value = ("""
apiVersion: v1
clusters:
- cluster:
    server: https://10.0.0.1:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
users:
- name: default
  user:
    token: test-token
""", False)  # (content, was_cached)

            with patch('fetch_k3s_config.merge_kubeconfig'):
                context_name, port, internal_ip, new_content, was_cached = fetch_and_merge_kubeconfig(
                    company="test-company",
                    host_alias="test-host",
                    host_info={"group": "k3s_cluster"},
                    ssh_config={"hostname": "10.0.0.1", "user": "ubuntu", "port": "22"},
                    remote_path="/etc/rancher/k3s/k3s.yaml",
                    target_port=6443,
                    port_range_start=16443,
                    port_range_size=10000,
                    ssh_client=mock_ssh
                )

        assert context_name == "test-company-test-host"
        assert 16443 <= port < 26443
        assert internal_ip == "10.0.0.1"
        assert "apiVersion: v1" in new_content
        assert was_cached is False

    def test_closes_ssh_connection_on_error(self):
        """Closes SSH connection when created internally and fetch fails."""
        mock_ssh = MagicMock()

        with patch('fetch_k3s_config.make_ssh_client', return_value=mock_ssh):
            with patch('fetch_k3s_config.get_internal_ip', side_effect=RuntimeError("Failed")):
                with pytest.raises(RuntimeError):
                    fetch_and_merge_kubeconfig(
                        company="test",
                        host_alias="host",
                        host_info={},
                        ssh_config={"hostname": "test.com", "user": "ubuntu", "port": "22"},
                        remote_path="/path",
                        target_port=6443,
                        port_range_start=16443,
                        port_range_size=10000
                    )

        mock_ssh.close.assert_called_once()

    def test_uses_custom_port_range(self):
        """Uses custom port range when specified."""
        mock_ssh = MagicMock()

        with patch('fetch_k3s_config.get_internal_ip', return_value="10.0.0.1"):
            with patch('fetch_k3s_config.fetch_remote_file_cached') as mock_fetch:
                mock_fetch.return_value = ("""
apiVersion: v1
clusters:
- cluster:
    server: https://10.0.0.1:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
users:
- name: default
  user:
    token: token
""", False)  # (content, was_cached)
                with patch('fetch_k3s_config.merge_kubeconfig'):
                    context_name, port, internal_ip, new_content, was_cached = fetch_and_merge_kubeconfig(
                        company="test",
                        host_alias="host",
                        host_info={},
                        ssh_config={},
                        remote_path="/path",
                        target_port=6443,
                        port_range_start=20000,
                        port_range_size=5000,
                        ssh_client=mock_ssh
                    )

        assert 20000 <= port < 25000


class TestMainScriptIntegration:
    """Integration tests for main script flow."""

    def test_main_script_loads_config_from_file(self):
        """Main script loads configuration from file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            config_file = Path(tmpdir) / "config.yaml"
            config_file.write_text("""
remote_k3s_config_path: /custom/k3s.yaml
k3s_api_port: 7443
port_range_start: 20000
port_range_size: 5000
""")

            # Would need to actually run main() with config,
            # but we're testing that config is loaded correctly
            from src.config import load_config
            config = load_config(str(config_file))

            assert config['remote_k3s_config_path'] == '/custom/k3s.yaml'
            assert config['k3s_api_port'] == 7443
            assert config['port_range_start'] == 20000
