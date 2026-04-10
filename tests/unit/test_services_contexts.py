"""Unit tests for context mutation helpers."""

from pytest_mock import MockerFixture

from src.bootstrap import ServiceContainer
from src.models import OperationError
from src.services.contexts import set_current_context


def test_set_current_context_delegates_to_application_use_case(
    mocker: MockerFixture,
) -> None:
    services = mocker.Mock(spec=ServiceContainer)
    services.switcher = object()
    build_services = mocker.patch(
        "src.services.contexts.build_service_container",
        return_value=services,
    )
    switch_context = mocker.patch(
        "src.services.contexts.set_current_context_use_case",
        return_value=None,
    )

    error = set_current_context(
        "acme-prod",
        require_confirmation=False,
        confirmed=True,
    )

    assert error is None
    build_services.assert_called_once_with()
    switch_context.assert_called_once_with(
        "acme-prod",
        switcher=services.switcher,
        require_confirmation=False,
        confirmed=True,
    )
