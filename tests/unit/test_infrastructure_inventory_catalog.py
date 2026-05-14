"""Tests for YAML inventory catalog and git refresher adapters."""

from __future__ import annotations

from pathlib import Path

from pytest_mock import MockerFixture

from src.infrastructure.adapters.inventory_catalog import (
    GitInventoryRefresher,
    YamlInventoryCatalog,
)


def test_yaml_inventory_catalog_returns_sorted_targets(tmp_path: Path) -> None:
    inventory_dir = tmp_path / "inventory"
    inventory_dir.mkdir()
    (inventory_dir / "acme_hosts.yml").write_text(
        "\n".join(
            [
                "all:",
                "  children:",
                "    k3s_cluster:",
                "      hosts:",
                "        prod:",
                "          ansible_host: 10.0.0.10",
                "        dev:",
                "          ansible_host: 10.0.0.11",
            ]
        )
    )

    targets = YamlInventoryCatalog().list_targets(inventory_dir)

    assert [t.context_name for t in targets] == ["acme-dev", "acme-prod"]


def test_yaml_inventory_catalog_populates_host_config(tmp_path: Path) -> None:
    inventory_dir = tmp_path / "inventory"
    inventory_dir.mkdir()
    (inventory_dir / "acme_hosts.yml").write_text(
        "\n".join(
            [
                "all:",
                "  children:",
                "    k3s_cluster:",
                "      hosts:",
                "        prod:",
                "          ansible_host: 10.0.0.10",
            ]
        )
    )

    targets = YamlInventoryCatalog().list_targets(inventory_dir)

    assert len(targets) == 1
    assert targets[0].host_config["ansible_host"] == "10.0.0.10"
    assert targets[0].group == "k3s_cluster"


def test_yaml_inventory_catalog_populates_group_vars(tmp_path: Path) -> None:
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
            ]
        )
    )

    targets = YamlInventoryCatalog().list_targets(inventory_dir)

    assert targets[0].group_vars["base_domain"] == "corp.example"
    assert targets[0].group_vars["gateway"] == "bastion.example"


def test_yaml_inventory_catalog_returns_empty_for_no_inventory_files(
    tmp_path: Path,
) -> None:
    inventory_dir = tmp_path / "inventory"
    inventory_dir.mkdir()

    targets = YamlInventoryCatalog().list_targets(inventory_dir)

    assert targets == []


def test_yaml_inventory_catalog_loads_multiple_companies(tmp_path: Path) -> None:
    inventory_dir = tmp_path / "inventory"
    inventory_dir.mkdir()
    (inventory_dir / "acme_hosts.yml").write_text(
        "\n".join(
            [
                "all:",
                "  children:",
                "    k3s_cluster:",
                "      hosts:",
                "        prod:",
                "          ansible_host: 10.0.0.10",
            ]
        )
    )
    (inventory_dir / "beta_hosts.yml").write_text(
        "\n".join(
            [
                "all:",
                "  children:",
                "    k3s_cluster:",
                "      hosts:",
                "        staging:",
                "          ansible_host: 10.0.1.10",
            ]
        )
    )

    targets = YamlInventoryCatalog().list_targets(inventory_dir)

    companies = {t.company for t in targets}
    assert companies == {"acme", "beta"}


def test_git_inventory_refresher_delegates_to_update_inventory_repo(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    update = mocker.patch(
        "src.infrastructure.adapters.inventory_catalog.update_inventory_repo",
        return_value=(True, "Already up to date."),
    )

    ok, msg = GitInventoryRefresher().refresh(tmp_path)

    assert ok is True
    assert msg == "Already up to date."
    update.assert_called_once_with(tmp_path)


def test_git_inventory_refresher_propagates_failure_result(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    mocker.patch(
        "src.infrastructure.adapters.inventory_catalog.update_inventory_repo",
        return_value=(False, "not a git repository"),
    )

    ok, msg = GitInventoryRefresher().refresh(tmp_path)

    assert ok is False
    assert "not a git repository" in msg
