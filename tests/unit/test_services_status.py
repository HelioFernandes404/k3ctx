"""Unit tests for the status service."""

from pathlib import Path

from pytest_mock import MockerFixture

from src.bootstrap import ServiceContainer
from src.services.status import list_context_status, validate_context_network


def test_list_context_status_delegates_to_application_use_case(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    services = mocker.Mock(spec=ServiceContainer)
    services.status_reader = object()
    build_services = mocker.patch(
        "src.services.status.build_service_container",
        return_value=services,
    )
    list_status = mocker.patch(
        "src.services.status.list_context_status_use_case",
        return_value=[{"name": "acme-prod"}],
    )

    result = list_context_status(tmp_path)

    assert result == [{"name": "acme-prod"}]
    build_services.assert_called_once_with(tmp_path)
    list_status.assert_called_once_with(services.status_reader)


def test_validate_context_network_delegates_to_application_use_case(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    services = mocker.Mock(spec=ServiceContainer)
    services.status_reader = object()
    build_services = mocker.patch(
        "src.services.status.build_service_container",
        return_value=services,
    )
    validate_network = mocker.patch(
        "src.services.status.validate_context_network_use_case",
        return_value={"context_name": "acme-prod", "ok": True},
    )

    result = validate_context_network("acme-prod", tmp_path)

    assert result == {"context_name": "acme-prod", "ok": True}
    build_services.assert_called_once_with(tmp_path)
    validate_network.assert_called_once_with("acme-prod", services.status_reader)
