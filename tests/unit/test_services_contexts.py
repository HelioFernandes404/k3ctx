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


def test_set_current_context_propagates_use_case_error(
    mocker: MockerFixture,
) -> None:
    kubectl_error = OperationError(
        code="kubectl_context_failed",
        message="Failed to switch kubectl context",
    )
    services = mocker.Mock(spec=ServiceContainer)
    services.switcher = object()
    mocker.patch("src.services.contexts.build_service_container", return_value=services)
    mocker.patch(
        "src.services.contexts.set_current_context_use_case",
        return_value=kubectl_error,
    )

    error = set_current_context(
        "acme-prod",
        require_confirmation=False,
        confirmed=True,
    )

    assert error is kubectl_error


def test_set_current_context_returns_confirmation_required_error(
    mocker: MockerFixture,
) -> None:
    confirm_error = OperationError(
        code="confirmation_required",
        message="Context switch requires explicit confirmation",
    )
    services = mocker.Mock(spec=ServiceContainer)
    services.switcher = object()
    mocker.patch("src.services.contexts.build_service_container", return_value=services)
    mocker.patch(
        "src.services.contexts.set_current_context_use_case",
        return_value=confirm_error,
    )

    error = set_current_context(
        "acme-prod",
        require_confirmation=True,
        confirmed=False,
    )

    assert error is not None
    assert error.code == "confirmation_required"
