"""Unit tests for config module."""

import pytest
import tempfile
import yaml
from pathlib import Path
from src.config import load_config, get_config_value, resolve_inventory_path


class TestLoadConfig:
    """Tests for load_config function."""

    def test_loads_valid_config_file(self):
        """Loads and parses valid YAML config file."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            yaml.dump({
                'remote_k3s_config_path': '/custom/path/k3s.yaml',
                'k3s_api_port': 7443,
                'ssh_key_path': '~/.ssh/custom_key',
                'port_range_start': 20000,
                'port_range_size': 5000
            }, f)
            config_path = f.name

        try:
            config = load_config(config_path)
            assert config['remote_k3s_config_path'] == '/custom/path/k3s.yaml'
            assert config['k3s_api_port'] == 7443
            assert config['port_range_start'] == 20000
            assert config['port_range_size'] == 5000
        finally:
            Path(config_path).unlink()

    def test_returns_empty_dict_for_missing_file(self):
        """Returns empty dict when config file doesn't exist."""
        config = load_config('/nonexistent/path/config.yaml')
        assert config == {}

    def test_merges_env_vars_with_config_file(self):
        """Environment variables override config file values."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            yaml.dump({
                'k3s_api_port': 7443,
                'remote_k3s_config_path': '/file/path'
            }, f)
            config_path = f.name

        try:
            import os
            original_port = os.environ.get('K3S_API_PORT')
            os.environ['K3S_API_PORT'] = '8443'

            config = load_config(config_path)
            assert config['k3s_api_port'] == 8443  # env var wins
            assert config['remote_k3s_config_path'] == '/file/path'  # from file

            if original_port:
                os.environ['K3S_API_PORT'] = original_port
            else:
                del os.environ['K3S_API_PORT']
        finally:
            Path(config_path).unlink()

    def test_inventory_path_can_be_overridden_by_env_var(self):
        """INVENTORY_PATH environment variable overrides file config."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            yaml.dump({'inventory_path': '/file/inventory/path'}, f)
            config_path = f.name

        try:
            import os
            original_inventory_path = os.environ.get('INVENTORY_PATH')
            os.environ['INVENTORY_PATH'] = '/env/inventory/path'

            config = load_config(config_path)
            assert config['inventory_path'] == '/env/inventory/path'

            if original_inventory_path:
                os.environ['INVENTORY_PATH'] = original_inventory_path
            else:
                del os.environ['INVENTORY_PATH']
        finally:
            Path(config_path).unlink()


class TestGetConfigValue:
    """Tests for get_config_value helper."""

    def test_returns_config_value_with_default(self):
        """Returns config value or default if missing."""
        config = {'key': 'value'}
        assert get_config_value(config, 'key', 'default') == 'value'
        assert get_config_value(config, 'missing', 'default') == 'default'

    def test_returns_config_value_without_default(self):
        """Returns config value or None if not specified."""
        config = {'key': 'value'}
        assert get_config_value(config, 'key') == 'value'
        assert get_config_value(config, 'missing') is None


class TestResolveInventoryPath:
    """Tests for inventory path resolution."""

    def test_uses_existing_configured_path(self):
        """Returns configured path when it exists."""
        with tempfile.TemporaryDirectory() as tmpdir:
            inventory_dir = Path(tmpdir) / "inventory"
            inventory_dir.mkdir()

            resolved = resolve_inventory_path(
                {'inventory_path': str(inventory_dir)},
                Path(tmpdir),
            )

            assert resolved == inventory_dir

    def test_falls_back_to_ancestor_ansible_inventory(self):
        """Finds ansible/inventory in an ancestor when configured path is stale."""
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            project_dir = root / ".custom-tools" / "k9s_setup"
            project_dir.mkdir(parents=True)
            ancestor_inventory = root / "ansible" / "inventory"
            ancestor_inventory.mkdir(parents=True)

            resolved = resolve_inventory_path(
                {'inventory_path': str(root / 'missing' / 'inventory')},
                project_dir,
            )

            assert resolved == ancestor_inventory


class TestConfigEdgeCases:
    """Tests for edge cases and error scenarios."""

    def test_handles_negative_numbers_in_env_vars(self):
        """Handles negative numbers in env vars gracefully."""
        import os
        original_port = os.environ.get('PORT_RANGE_START')

        try:
            os.environ['PORT_RANGE_START'] = '-1000'
            config = load_config('/nonexistent')
            # Should convert to int (negative)
            assert config.get('port_range_start') == -1000
            assert isinstance(config.get('port_range_start'), int)
        finally:
            if original_port:
                os.environ['PORT_RANGE_START'] = original_port
            elif 'PORT_RANGE_START' in os.environ:
                del os.environ['PORT_RANGE_START']

    def test_converts_numeric_strings_in_yaml(self):
        """Converts numeric strings from YAML to int type."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            # Write YAML with string numbers
            yaml.dump({
                'k3s_api_port': '7443',
                'port_range_start': '20000',
                'port_range_size': '5000'
            }, f)
            config_path = f.name

        try:
            config = load_config(config_path)
            assert isinstance(config['k3s_api_port'], int)
            assert isinstance(config['port_range_start'], int)
            assert isinstance(config['port_range_size'], int)
            assert config['k3s_api_port'] == 7443
        finally:
            Path(config_path).unlink()

    def test_handles_malformed_yaml(self):
        """Handles malformed YAML gracefully with warning."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            f.write("invalid: yaml: content: [")  # Malformed YAML
            config_path = f.name

        try:
            config = load_config(config_path)
            # Should return empty dict on parse error
            assert config == {}
        finally:
            Path(config_path).unlink()

    def test_handles_non_numeric_env_var_for_port(self):
        """Handles non-numeric env var values gracefully."""
        import os
        original_port = os.environ.get('K3S_API_PORT')

        try:
            os.environ['K3S_API_PORT'] = 'not_a_number'
            config = load_config('/nonexistent')
            # Should keep as string if conversion fails
            assert config.get('k3s_api_port') == 'not_a_number'
            assert isinstance(config.get('k3s_api_port'), str)
        finally:
            if original_port:
                os.environ['K3S_API_PORT'] = original_port
            elif 'K3S_API_PORT' in os.environ:
                del os.environ['K3S_API_PORT']

    def test_yaml_numeric_fields_normalized_before_env_override(self):
        """YAML numeric fields are normalized before env vars override."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            yaml.dump({'k3s_api_port': '6443'}, f)
            config_path = f.name

        try:
            import os
            original_port = os.environ.get('K3S_API_PORT')

            try:
                # No env var, just YAML with string number
                if 'K3S_API_PORT' in os.environ:
                    del os.environ['K3S_API_PORT']

                config = load_config(config_path)
                # Should be int from YAML normalization
                assert config['k3s_api_port'] == 6443
                assert isinstance(config['k3s_api_port'], int)
            finally:
                if original_port:
                    os.environ['K3S_API_PORT'] = original_port
        finally:
            Path(config_path).unlink()

    def test_inventory_path_from_config(self):
        """Test inventory_path can be loaded from config file."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            yaml.dump({'inventory_path': '/custom/inventory/path'}, f)
            config_path = f.name

        try:
            config = load_config(config_path)
            inventory_path = get_config_value(config, 'inventory_path', './inventory')
            assert inventory_path == '/custom/inventory/path'
        finally:
            Path(config_path).unlink()

    def test_inventory_path_default_when_not_in_config(self):
        """Test inventory_path returns default when not in config."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            yaml.dump({'k3s_api_port': 6443}, f)
            config_path = f.name

        try:
            config = load_config(config_path)
            inventory_path = get_config_value(config, 'inventory_path', './inventory')
            assert inventory_path == './inventory'
        finally:
            Path(config_path).unlink()
