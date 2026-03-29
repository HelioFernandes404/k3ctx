"""Unit tests for kubeconfig module."""

import pytest
import tempfile
import yaml
from pathlib import Path
from src.kubeconfig import update_kubeconfig_server, merge_kubeconfig


class TestUpdateKubeconfigServer:
    """Tests for update_kubeconfig_server function."""

    def test_updates_server_with_direct_ip(self):
        """Updates server URL with direct IP and port."""
        kubeconfig_yaml = """
apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: default
"""
        result = update_kubeconfig_server(kubeconfig_yaml, "10.0.0.1", 6443)

        data = yaml.safe_load(result)
        assert data["clusters"][0]["cluster"]["server"] == "https://10.0.0.1:6443"

    def test_updates_server_with_localhost_tunnel(self):
        """Updates server URL to use localhost tunnel."""
        kubeconfig_yaml = """
apiVersion: v1
clusters:
- cluster:
    server: https://192.168.1.100:6443
  name: default
"""
        result = update_kubeconfig_server(
            kubeconfig_yaml,
            "192.168.1.100",
            6443,
            use_localhost=True,
            local_port=16443
        )

        data = yaml.safe_load(result)
        assert data["clusters"][0]["cluster"]["server"] == "https://127.0.0.1:16443"

    def test_raises_error_for_invalid_kubeconfig(self):
        """Raises RuntimeError for YAML without clusters key."""
        invalid_yaml = """
apiVersion: v1
kind: Config
"""
        with pytest.raises(RuntimeError, match="no 'clusters' key"):
            update_kubeconfig_server(invalid_yaml, "10.0.0.1", 6443)

    def test_preserves_other_kubeconfig_fields(self):
        """Preserves other fields in kubeconfig."""
        kubeconfig_yaml = """
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: CERTDATA
    server: https://old:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
"""
        result = update_kubeconfig_server(kubeconfig_yaml, "10.0.0.1", 6443)

        data = yaml.safe_load(result)
        assert "certificate-authority-data" in data["clusters"][0]["cluster"]
        assert len(data["contexts"]) == 1


class TestMergeKubeconfig:
    """Tests for merge_kubeconfig function."""

    def test_creates_new_kubeconfig_when_missing(self):
        """Creates new kubeconfig file when it doesn't exist."""
        with tempfile.TemporaryDirectory() as tmpdir:
            # Override HOME to use temp directory
            kube_dir = Path(tmpdir) / ".kube"
            kubeconfig_path = kube_dir / "config"

            new_config = """
apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:16443
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
"""

            with patch_home(tmpdir):
                result_path = merge_kubeconfig(new_config, "test-context")

            assert result_path.exists()
            with open(result_path) as f:
                data = yaml.safe_load(f)

            assert data["current-context"] == "test-context"
            assert data["clusters"][0]["name"] == "test-context"

    def test_merges_into_existing_kubeconfig(self):
        """Merges new context into existing kubeconfig."""
        with tempfile.TemporaryDirectory() as tmpdir:
            kube_dir = Path(tmpdir) / ".kube"
            kube_dir.mkdir()
            kubeconfig_path = kube_dir / "config"

            # Create existing kubeconfig
            existing = {
                "apiVersion": "v1",
                "kind": "Config",
                "clusters": [{"name": "existing-cluster", "cluster": {"server": "https://old:6443"}}],
                "contexts": [{"name": "existing-context", "context": {"cluster": "existing-cluster", "user": "existing-user"}}],
                "users": [{"name": "existing-user", "user": {"token": "old-token"}}],
                "current-context": "existing-context"
            }

            with open(kubeconfig_path, 'w') as f:
                yaml.dump(existing, f)

            new_config = """
apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:16443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
users:
- name: default
  user:
    token: new-token
"""

            with patch_home(tmpdir):
                merge_kubeconfig(new_config, "new-context")

            with open(kubeconfig_path) as f:
                merged = yaml.safe_load(f)

            # Should have both contexts
            assert len(merged["clusters"]) == 2
            assert len(merged["contexts"]) == 2
            assert merged["current-context"] == "new-context"

    def test_replaces_existing_context_with_same_name(self):
        """Replaces existing context if name matches."""
        with tempfile.TemporaryDirectory() as tmpdir:
            kube_dir = Path(tmpdir) / ".kube"
            kube_dir.mkdir()
            kubeconfig_path = kube_dir / "config"

            # Create existing kubeconfig
            existing = {
                "apiVersion": "v1",
                "clusters": [{"name": "test-context", "cluster": {"server": "https://old:6443"}}],
                "contexts": [{"name": "test-context", "context": {"cluster": "test-context", "user": "test-context"}}],
                "users": [{"name": "test-context", "user": {"token": "old-token"}}]
            }

            with open(kubeconfig_path, 'w') as f:
                yaml.dump(existing, f)

            new_config = """
apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:16443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
users:
- name: default
  user:
    token: new-token
"""

            with patch_home(tmpdir):
                merge_kubeconfig(new_config, "test-context")

            with open(kubeconfig_path) as f:
                merged = yaml.safe_load(f)

            # Should have only 1 context (replaced)
            assert len(merged["clusters"]) == 1
            assert merged["clusters"][0]["cluster"]["server"] == "https://127.0.0.1:16443"

    def test_creates_backup_of_existing_config(self):
        """Creates backup file before overwriting kubeconfig."""
        with tempfile.TemporaryDirectory() as tmpdir:
            kube_dir = Path(tmpdir) / ".kube"
            kube_dir.mkdir()
            kubeconfig_path = kube_dir / "config"
            backup_path = kube_dir / "config.bak"

            # Create existing kubeconfig
            existing = {"apiVersion": "v1", "clusters": [], "contexts": [], "users": []}
            with open(kubeconfig_path, 'w') as f:
                yaml.dump(existing, f)

            new_config = """
apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:16443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
users:
- name: default
  user:
    token: test
"""

            with patch_home(tmpdir):
                merge_kubeconfig(new_config, "test-context")

            assert backup_path.exists()


# Helper to patch HOME environment variable
from unittest.mock import patch

def patch_home(tmpdir):
    """Context manager to temporarily override HOME directory."""
    return patch.dict('os.environ', {'HOME': str(tmpdir)})
