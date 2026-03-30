"""Unit tests for core typed models."""

from pathlib import Path
from typing import Any, cast

import pytest

from src.models import (
    ClusterTarget,
    ConnectResult,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)


def test_effective_config_exposes_expected_fields() -> None:
    config = EffectiveConfig(
        inventory_path=Path("/tmp/inventory"),
        ssh_config_path="~/.ssh/config",
        ssh_key_path="~/.ssh/id_ed25519",
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )

    assert config.inventory_path == Path("/tmp/inventory")
    assert config.ssh_config_path == "~/.ssh/config"
    assert config.ssh_key_path == "~/.ssh/id_ed25519"
    assert config.remote_k3s_config_path == "/etc/rancher/k3s/k3s.yaml"
    assert config.k3s_api_port == 6443
    assert config.port_range_start == 16443
    assert config.port_range_size == 10000


def test_network_requirement_none_factory_returns_empty_requirement() -> None:
    requirement = NetworkRequirement.none()

    assert requirement.type is None
    assert requirement.network_range is None
    assert requirement.needs_vpn is False


def test_cluster_target_exposes_context_name() -> None:
    target = ClusterTarget(company="acme", host_alias="prod", group="k3s_cluster")

    assert target.context_name == "acme-prod"


def test_cluster_target_host_config_is_protected_from_mutation() -> None:
    nested_network = {"dns": "10.96.0.10"}
    nested_group_var = {"enabled": True}
    host_config = {
        "ansible_host": "10.0.0.10",
        "network": nested_network,
    }
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config=host_config,
        group_vars={"proxy": nested_group_var},
    )

    nested_network["dns"] = "10.96.0.20"
    nested_group_var["enabled"] = False
    host_config["ansible_host"] = "10.0.0.20"

    assert target.host_config["ansible_host"] == "10.0.0.10"
    assert target.host_config["network"]["dns"] == "10.96.0.10"
    assert target.group_vars["proxy"]["enabled"] is True

    with pytest.raises(TypeError):
        cast(Any, target.host_config)["ansible_host"] = "10.0.0.30"

    with pytest.raises(TypeError):
        cast(Any, target.host_config["network"])["dns"] = "10.96.0.30"

    with pytest.raises(TypeError):
        cast(Any, target.group_vars)["proxy"] = {"enabled": False}

    with pytest.raises(TypeError):
        cast(Any, target.group_vars["proxy"])["enabled"] = False

    assert dict(target.host_config) == {
        "ansible_host": "10.0.0.10",
        "network": target.host_config["network"],
    }
    assert dict(target.group_vars) == {
        "proxy": target.group_vars["proxy"],
    }


def test_connect_result_to_public_dict_includes_safe_summary() -> None:
    result = ConnectResult(
        success=True,
        context_name="acme-prod",
        local_port=16443,
        internal_ip="10.0.0.10",
        tunnel_pid=4242,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
    )

    public = result.to_public_dict()

    assert public["success"] is True
    assert public["context_name"] == "acme-prod"
    assert public["local_port"] == 16443
    assert public["internal_ip"] == "10.0.0.10"
    assert public["tunnel_pid"] == 4242
    assert public["used_cache"] is False
    assert public["network_requirement"] == {
        "type": None,
        "network_range": None,
        "needs_vpn": False,
    }
    assert public["error"] is None


def test_operation_error_to_public_dict_redacts_detail() -> None:
    error = OperationError(
        code="ssh_connect_failed",
        message="SSH connection failed",
        detail="token=secret",
        retryable=True,
    )

    public = error.to_public_dict()

    assert public["code"] == "ssh_connect_failed"
    assert public["message"] == "SSH connection failed"
    assert public["retryable"] is True
    assert "detail" not in public
    assert "secret" not in str(public)


def test_connect_result_requires_error_to_match_success_state() -> None:
    with pytest.raises(ValueError, match="success=True requires error=None"):
        ConnectResult(
            success=True,
            context_name="acme-prod",
            local_port=16443,
            internal_ip="10.0.0.10",
            tunnel_pid=4242,
            used_cache=False,
            network_requirement=NetworkRequirement.none(),
            error=OperationError(code="unexpected", message="unexpected"),
        )

    with pytest.raises(ValueError, match="success=False requires error to be set"):
        ConnectResult(
            success=False,
            context_name="acme-prod",
            local_port=None,
            internal_ip=None,
            tunnel_pid=None,
            used_cache=False,
            network_requirement=NetworkRequirement.none(),
        )


def test_connect_result_to_public_dict_includes_error_payload() -> None:
    error = OperationError(
        code="ssh_connect_failed",
        message="SSH connection failed",
        detail="token=secret",
        retryable=True,
    )
    result = ConnectResult(
        success=False,
        context_name="acme-prod",
        local_port=None,
        internal_ip=None,
        tunnel_pid=None,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
        error=error,
    )

    public = result.to_public_dict()

    assert public["success"] is False
    assert public["error"] == {
        "code": "ssh_connect_failed",
        "message": "SSH connection failed",
        "retryable": True,
    }
