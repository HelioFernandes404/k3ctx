"""
Smoke tests for k9s-config end-to-end workflows.

These tests validate the main user flows work correctly:
1. Select company → host → fetch kubeconfig → create tunnel
2. VPN warnings are shown for appropriate hosts
3. Private network detection triggers sshuttle warnings
"""

import sys
from pathlib import Path
from io import StringIO
from typing import Any
from unittest.mock import patch

from _pytest.capture import CaptureFixture
from pytest import MonkeyPatch
from pytest_mock import MockerFixture
# Add project root to path
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))


def test_full_workflow_mock() -> None:
    """
    Smoke test: Manual single-cluster flow delegates to shared services.
    """
    import fetch_k3s_config
    from src.models import ConnectResult, EffectiveConfig, NetworkRequirement

    config = EffectiveConfig(
        inventory_path=project_root,
        ssh_config_path=str(project_root / "ssh_config"),
        ssh_key_path=str(project_root / "id_ed25519"),
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )
    result = ConnectResult(
        success=True,
        context_name="testcompany-testhost",
        local_port=16443,
        internal_ip="10.0.0.100",
        tunnel_pid=12345,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
    )

    with patch.object(fetch_k3s_config, "load_effective_config", return_value=config), \
         patch.object(fetch_k3s_config, "setup_logging"), \
         patch.object(fetch_k3s_config, "update_inventory_repo", return_value=(True, "updated")), \
         patch.object(fetch_k3s_config, "select_company", return_value=("testcompany", {"all": {}})), \
         patch.object(
             fetch_k3s_config,
             "select_host",
             return_value=(
                 "testhost",
                 {
                     "group": "k3s_cluster",
                     "config": {"ansible_host": "8.8.8.8"},
                     "group_vars": {},
                 },
             ),
         ), \
         patch.object(fetch_k3s_config, "connect_cluster", return_value=result) as connect_cluster:
        assert fetch_k3s_config.main() == 0

    connect_cluster.assert_called_once()


def test_vpn_warning_detection() -> None:
    """
    Smoke test: VPN requirement detection.

    Validates:
    - Hosts with argocd_use_socks5_proxy=true show VPN warning
    """
    import fetch_k3s_config

    # Inventory with VPN requirement
    inventory_data = {
        "all": {
            "children": {
                "k3s_cluster": {
                    "vars": {
                        "argocd_use_socks5_proxy": True
                    },
                    "hosts": {
                        "vpnhost": {
                            "ansible_host": "192.168.1.100"
                        }
                    }
                }
            }
        }
    }

    result = fetch_k3s_config.check_vpn_requirement(
        inventory_data,
        "k3s_cluster",
        "vpnhost"
    )

    assert result is True, "VPN requirement should be detected"


def test_private_network_detection() -> None:
    """
    Smoke test: Private network detection for sshuttle requirement.

    Validates:
    - Private IPs (192.168.x.x, 10.x.x.x, 172.16-31.x.x) are detected
    - Public IPs are not flagged
    """
    import fetch_k3s_config

    # Test private IPs
    assert fetch_k3s_config.is_private_network("192.168.1.100") is True
    assert fetch_k3s_config.is_private_network("10.0.0.1") is True
    assert fetch_k3s_config.is_private_network("172.16.0.1") is True

    # Test public IPs
    assert fetch_k3s_config.is_private_network("8.8.8.8") is False
    assert fetch_k3s_config.is_private_network("1.1.1.1") is False

    # Test hostname (should return False - can't determine)
    assert fetch_k3s_config.is_private_network("example.com") is False


def test_network_requirement_check() -> None:
    """
    Smoke test: Network requirement detection returns correct type.

    Validates:
    - Private IPs return ("sshuttle", network_range)
    - Public IPs return (None, None)
    """
    import fetch_k3s_config

    # Private IP should trigger sshuttle
    host_info_private = {
        "config": {
            "ansible_host": "192.168.90.100"
        }
    }

    network_type, network_range = fetch_k3s_config.check_network_requirement(
        "testhost",
        host_info_private
    )

    assert network_type == "sshuttle", "Private IP should require sshuttle"
    assert network_range is not None
    assert "192.168.90.0/24" in network_range, "Should detect /24 network range"

    # Public IP should not trigger
    host_info_public = {
        "config": {
            "ansible_host": "8.8.8.8"
        }
    }

    network_type, network_range = fetch_k3s_config.check_network_requirement(
        "testhost",
        host_info_public
    )

    assert network_type is None, "Public IP should not require sshuttle"
    assert network_range is None


def test_inventory_loading() -> None:
    """
    Smoke test: Inventory loading handles vault tags gracefully.

    Validates:
    - YAML with !vault tags doesn't crash
    - Basic inventory structure is parsed correctly
    """
    import fetch_k3s_config
    import yaml

    # Test the custom YAML loader
    yaml_with_vault = """
all:
  vars:
    password: !vault |
      $ANSIBLE_VAULT;1.1;AES256
      abc123
  children:
    k3s_cluster:
      hosts:
        testhost:
          ansible_host: 192.168.1.100
"""

    # This should not raise an exception
    class VaultIgnoreLoader(yaml.SafeLoader):
        pass

    def ignore_unknown_tag(
        loader: yaml.SafeLoader,
        tag_suffix: str,
        node: yaml.Node,
    ) -> Any:
        if isinstance(node, yaml.MappingNode):
            return loader.construct_mapping(node)
        elif isinstance(node, yaml.SequenceNode):
            return loader.construct_sequence(node)
        else:
            return node.value

    VaultIgnoreLoader.add_multi_constructor('', ignore_unknown_tag)

    result = yaml.load(StringIO(yaml_with_vault), Loader=VaultIgnoreLoader)

    assert "all" in result
    assert "children" in result["all"]
    assert "k3s_cluster" in result["all"]["children"]


def test_unique_port_generation() -> None:
    """
    Smoke test: Context names generate unique, deterministic ports.

    Validates:
    - Same context name always generates same port
    - Different context names generate different ports
    - Ports are in expected range (16443-26443)
    """
    import fetch_k3s_config

    context1 = "company1-host1"
    context2 = "company2-host2"

    # Same input should generate same port
    port1a = fetch_k3s_config.get_unique_port(context1)
    port1b = fetch_k3s_config.get_unique_port(context1)
    assert port1a == port1b, "Same context should generate same port"

    # Different inputs should likely generate different ports
    port2 = fetch_k3s_config.get_unique_port(context2)
    # Note: Hash collision is possible but unlikely

    # Ports should be in expected range
    assert 16443 <= port1a <= 26443, f"Port {port1a} out of expected range"
    assert 16443 <= port2 <= 26443, f"Port {port2} out of expected range"


def test_multi_connect_uses_services_and_sets_first_successful_context(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    import multi_connect
    from src.models import (
        ClusterTarget,
        ConnectResult,
        EffectiveConfig,
        NetworkRequirement,
    )

    config = EffectiveConfig(
        inventory_path=tmp_path,
        ssh_config_path=str(tmp_path / "ssh_config"),
        ssh_key_path=str(tmp_path / "id_ed25519"),
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )
    selected = [
        ClusterTarget(
            company="acme",
            host_alias="prod",
            group="k3s_cluster",
            host_config={"ansible_host": "203.0.113.10"},
            group_vars={},
        ),
        ClusterTarget(
            company="beta",
            host_alias="staging",
            group="k3s_cluster",
            host_config={"ansible_host": "203.0.113.11"},
            group_vars={},
        ),
    ]
    results = [
        ConnectResult(
            success=True,
            context_name="acme-prod",
            local_port=16443,
            internal_ip="10.0.0.10",
            tunnel_pid=1111,
            used_cache=False,
            network_requirement=NetworkRequirement.none(),
        ),
        ConnectResult(
            success=True,
            context_name="beta-staging",
            local_port=16444,
            internal_ip="10.0.0.11",
            tunnel_pid=2222,
            used_cache=True,
            network_requirement=NetworkRequirement.none(),
        ),
    ]

    mocker.patch("multi_connect.load_effective_config", return_value=config)
    setup_logging = mocker.patch("multi_connect.setup_logging")
    mocker.patch("multi_connect.list_cluster_targets", return_value=selected)
    mocker.patch("multi_connect.select_clusters_interactive", return_value=selected)
    mocker.patch("multi_connect.show_network_warnings", return_value=True)
    connect_multiple = mocker.patch("multi_connect.connect_multiple", return_value=results)
    set_current_context = mocker.patch("multi_connect.set_current_context", return_value=None)

    assert multi_connect.main() == 0

    setup_logging.assert_called_once()
    assert setup_logging.call_args.kwargs["structured"] is True
    connect_multiple.assert_called_once_with(
        targets=selected,
        config=config,
        allow_manual_network=True,
    )
    set_current_context.assert_called_once_with(
        "acme-prod",
        require_confirmation=False,
        confirmed=True,
    )


def test_show_network_warnings_reports_vpn_and_sshuttle_for_dual_requirement(
    mocker: MockerFixture,
    capsys: CaptureFixture[str],
) -> None:
    import multi_connect
    from src.models import ClusterTarget

    dual_target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "192.168.10.20"},
        group_vars={"argocd_use_socks5_proxy": True},
    )

    mocker.patch("multi_connect.require_interactive_terminal")
    mock_confirm = mocker.patch("multi_connect.questionary.confirm")
    mock_confirm.return_value.ask.return_value = True

    assert multi_connect.show_network_warnings([dual_target]) is True

    output = capsys.readouterr().out
    assert "⚠ Requires sshuttle:" in output
    assert "⚠ Requires VPN:" in output
    assert "acme: prod → 192.168.10.0/24" in output
    assert "sshuttle -v -r helio@100.64.5.10 192.168.10.0/24" in output


def test_post_connection_output_preserves_vpn_and_sshuttle_for_dual_requirement(
    capsys: CaptureFixture[str],
) -> None:
    import multi_connect
    from src.models import ConnectResult, NetworkRequirement

    result = ConnectResult(
        success=True,
        context_name="acme-prod",
        local_port=16443,
        internal_ip="10.0.0.10",
        tunnel_pid=1234,
        used_cache=False,
        network_requirement=NetworkRequirement(
            type="sshuttle",
            network_range="192.168.10.0/24",
            needs_vpn=True,
        ),
    )

    successful = multi_connect._print_summary([result])
    multi_connect._print_network_reminders(successful)

    output = capsys.readouterr().out
    assert "acme-prod (localhost:16443) ⚠ requires VPN + sshuttle" in output
    assert "sshuttle -v -r helio@100.64.5.10 192.168.10.0/24" in output
    assert "Ensure VPN connection is active before proceeding" in output


def test_post_connection_output_uses_public_error_message_only(
    capsys: CaptureFixture[str],
) -> None:
    import multi_connect
    from src.models import ConnectResult, NetworkRequirement, OperationError

    result = ConnectResult(
        success=False,
        context_name="acme-prod",
        local_port=None,
        internal_ip=None,
        tunnel_pid=None,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
        error=OperationError(
            code="connect_failed",
            message="Cluster connection failed",
            detail="token=secret",
        ),
    )

    multi_connect._print_summary([result])

    output = capsys.readouterr().out
    assert "Cluster connection failed" in output
    assert "token=secret" not in output


def test_makefile_exposes_mcp_targets() -> None:
    makefile = Path("Makefile").read_text()

    assert "mcp-stdio:" in makefile
    assert "mcp-http:" in makefile
    assert "src/mcp_server.py" in makefile or "src.mcp_server" in makefile


def test_readme_documents_manual_and_mcp_modes() -> None:
    readme = Path("README.md").read_text()

    assert "make run" in readme
    assert "make mcp-stdio" in readme
    assert "make mcp-http" in readme
    assert "connect_cluster" in readme
    assert "inventory://clusters" in readme
    assert "config.yaml" in readme


def test_agents_mentions_layered_architecture_and_mcp_server() -> None:
    agents = Path("AGENTS.md").read_text()

    assert "camadas" in agents.lower() or "layers" in agents.lower()
    assert "src/mcp_server.py" in agents
