"""Unit tests for the inventory service."""

from pathlib import Path

from src.services.inventory_service import list_cluster_targets
def test_list_cluster_targets_returns_structured_targets_with_group_vars(
    tmp_path: Path,
) -> None:
    inventory_dir = tmp_path / "inventory"
    inventory_dir.mkdir()
    (inventory_dir / "acme_hosts.yml").write_text(
        "\n".join(
            [
                "all:",
                "  vars:",
                "    base_domain: corp.example",
                "  children:",
                "    k3s_cluster:",
                "      vars:",
                "        gateway: bastion.example",
                "      hosts:",
                "        prod:",
                "          ansible_host: 10.0.0.10",
                "        dev:",
                "          ansible_host: 10.0.0.11",
            ]
        )
    )

    targets = list_cluster_targets(inventory_dir)

    assert [target.context_name for target in targets] == ["acme-dev", "acme-prod"]
    assert targets[0].group == "k3s_cluster"
    assert targets[0].group_vars["base_domain"] == "corp.example"
    assert targets[0].group_vars["gateway"] == "bastion.example"
    assert targets[0].host_config["ansible_host"] == "10.0.0.11"
