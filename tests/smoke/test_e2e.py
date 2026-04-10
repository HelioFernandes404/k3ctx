"""Smoke tests for end-to-end workflows and compatibility contracts."""

from pathlib import Path
from io import StringIO
from typing import Any

from _pytest.capture import CaptureFixture


def test_vpn_warning_detection() -> None:
    from src.domain.network import check_vpn_requirement

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

    result = check_vpn_requirement(
        inventory_data,
        "k3s_cluster",
        "vpnhost"
    )

    assert result is True, "VPN requirement should be detected"


def test_private_network_detection() -> None:
    from src.domain.network import is_private_network

    assert is_private_network("192.168.1.100") is True
    assert is_private_network("10.0.0.1") is True
    assert is_private_network("172.16.0.1") is True

    assert is_private_network("8.8.8.8") is False
    assert is_private_network("1.1.1.1") is False

    assert is_private_network("example.com") is False


def test_network_requirement_check() -> None:
    from src.domain.network import check_network_requirement

    host_info_private = {
        "config": {
            "ansible_host": "192.168.90.100"
        }
    }

    network_type, network_range = check_network_requirement(
        "testhost",
        host_info_private
    )

    assert network_type == "sshuttle", "Private IP should require sshuttle"
    assert network_range is not None
    assert "192.168.90.0/24" in network_range, "Should detect /24 network range"

    host_info_public = {
        "config": {
            "ansible_host": "8.8.8.8"
        }
    }

    network_type, network_range = check_network_requirement(
        "testhost",
        host_info_public
    )

    assert network_type is None, "Public IP should not require sshuttle"
    assert network_range is None


def test_inventory_loading() -> None:
    import yaml

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
    from src.tunnel import get_unique_port

    context1 = "company1-host1"
    context2 = "company2-host2"

    port1a = get_unique_port(context1)
    port1b = get_unique_port(context1)
    assert port1a == port1b, "Same context should generate same port"

    port2 = get_unique_port(context2)

    assert 16443 <= port1a <= 26443, f"Port {port1a} out of expected range"
    assert 16443 <= port2 <= 26443, f"Port {port2} out of expected range"


def test_post_connection_output_preserves_vpn_and_sshuttle_for_dual_requirement(
    capsys: CaptureFixture[str],
) -> None:
    from src.interfaces.cli.presenters import (
        print_multi_summary,
        print_network_reminders,
    )
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

    successful = print_multi_summary([result])
    print_network_reminders(successful)

    output = capsys.readouterr().out
    assert "acme-prod (localhost:16443) ⚠ requires VPN + sshuttle" in output
    assert "sshuttle -v -r helio@100.64.5.10 192.168.10.0/24" in output
    assert "Ensure VPN connection is active before proceeding" in output


def test_post_connection_output_uses_public_error_message_only(
    capsys: CaptureFixture[str],
) -> None:
    from src.interfaces.cli.presenters import print_multi_summary
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

    print_multi_summary([result])

    output = capsys.readouterr().out
    assert "Cluster connection failed" in output
    assert "token=secret" not in output


def test_makefile_exposes_mcp_targets() -> None:
    makefile = Path("Makefile").read_text()

    assert "http:" in makefile
    assert "k3s-context-tunnel-manager-http" in makefile
    assert "mcp-stdio:" in makefile
    assert "mcp-http:" in makefile
    assert "k3s-context-tunnel-manager-mcp-stdio" in makefile
    assert "src/mcp_server.py" in makefile or "src.mcp_server" in makefile


def test_readme_documents_manual_and_mcp_modes() -> None:
    readme = Path("README.md").read_text()

    assert "make run" in readme
    assert "context-tunnel-manager init" in readme
    assert "context-tunnel-manager k9s" in readme
    assert "context-tunnel-manager tunnel-list" in readme
    assert "make http" in readme
    assert "GET /config" in readme
    assert "POST /connect" in readme
    assert "make mcp-stdio" in readme
    assert "k3s-context-tunnel-manager-mcp-stdio" in readme
    assert "make mcp-http" in readme
    assert "connect_cluster" in readme
    assert "inventory://clusters" in readme
    assert "config.yaml" in readme
    assert "~/.local/share/k3s-context-tunnel-manager/yaml/" in readme


def test_agents_mentions_layered_architecture_and_mcp_server() -> None:
    agents = Path("AGENTS.md").read_text()

    assert "camadas" in agents.lower() or "layers" in agents.lower()
    assert "src/mcp_server.py" in agents
    assert "~/.local/share/k3s-context-tunnel-manager/yaml/" in agents


def test_example_config_uses_neutral_examples_path() -> None:
    assert Path("examples/config/config.yaml").exists()
    assert not Path(".k9s-config-example").exists()
