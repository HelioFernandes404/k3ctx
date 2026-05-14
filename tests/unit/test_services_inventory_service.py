"""Unit tests for the inventory service."""

from pathlib import Path

import pytest
from pytest_mock import MockerFixture

from src.bootstrap import ServiceContainer
from src.domain.models import ClusterTarget
from src.services.inventory_service import list_cluster_targets


def test_list_cluster_targets_delegates_to_application_use_case(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )
    services = mocker.Mock(spec=ServiceContainer)
    services.catalog = object()
    build_services = mocker.patch(
        "src.services.inventory_service.build_service_container",
        return_value=services,
    )
    list_targets = mocker.patch(
        "src.services.inventory_service.list_cluster_targets_use_case",
        return_value=[target],
    )

    result = list_cluster_targets(tmp_path)

    assert result == [target]
    build_services.assert_called_once_with()
    list_targets.assert_called_once_with(tmp_path, services.catalog)


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


def test_list_cluster_targets_returns_empty_list_for_empty_inventory(
    tmp_path: Path,
) -> None:
    inventory_dir = tmp_path / "inventory"
    inventory_dir.mkdir()

    targets = list_cluster_targets(inventory_dir)

    assert targets == []


def test_list_cluster_targets_propagates_exception_from_use_case(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    services = mocker.Mock(spec=ServiceContainer)
    services.catalog = object()
    mocker.patch(
        "src.services.inventory_service.build_service_container",
        return_value=services,
    )
    mocker.patch(
        "src.services.inventory_service.list_cluster_targets_use_case",
        side_effect=FileNotFoundError("inventory not found"),
    )

    with pytest.raises(FileNotFoundError, match="inventory not found"):
        list_cluster_targets(tmp_path)
