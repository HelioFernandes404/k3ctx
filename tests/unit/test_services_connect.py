"""Unit tests for the shared cluster connection service."""

from pathlib import Path
from unittest.mock import MagicMock

from pytest_mock import MockerFixture

from src.models import ClusterTarget, EffectiveConfig
from src.services.connect import (
    _ensure_tunnel,
    _prepare_local_kubeconfig,
    _resolve_network_requirement,
    connect_cluster,
    connect_multiple,
)


def build_config(tmp_path: Path) -> EffectiveConfig:
    return EffectiveConfig(
        inventory_path=tmp_path / "inventory",
        ssh_config_path="~/.ssh/config",
        ssh_key_path="~/.ssh/id_ed25519",
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )


def test_resolve_network_requirement_returns_blocking_error_when_manual_network_is_disabled(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="vpn",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )
    mocker.patch(
        "src.services.connect.detect_network_requirement",
        return_value=("sshuttle", "10.0.0.0/24", True),
    )

    requirement, error = _resolve_network_requirement(
        target,
        allow_manual_network=False,
    )

    assert requirement.type == "sshuttle"
    assert requirement.network_range == "10.0.0.0/24"
    assert requirement.needs_vpn is True
    assert error is not None
    assert error.code == "network_requirement_unmet"


def test_prepare_local_kubeconfig_returns_internal_ip_port_and_cache_usage(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )
    ssh_client = MagicMock()

    mocker.patch("src.services.connect.get_internal_ip", return_value="10.0.0.10")
    fetch_remote_file_cached = mocker.patch(
        "src.services.connect.fetch_remote_file_cached",
        return_value=("apiVersion: v1\nclusters: []\n", True),
    )
    mocker.patch(
        "src.services.connect.update_kubeconfig_server",
        return_value="apiVersion: v1\n",
    )
    merge_kubeconfig = mocker.patch("src.services.connect.merge_kubeconfig")
    mocker.patch("src.services.connect.get_unique_port", return_value=20001)

    internal_ip, local_port, used_cache = _prepare_local_kubeconfig(
        target,
        config,
        ssh_client,
    )

    assert internal_ip == "10.0.0.10"
    assert local_port == 20001
    assert used_cache is True
    fetch_remote_file_cached.assert_called_once()
    merge_kubeconfig.assert_called_once_with("apiVersion: v1\n", "acme-prod")


def test_ensure_tunnel_reuses_existing_pid_without_creating_new_tunnel(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )

    mocker.patch("src.services.connect.is_tunnel_running", return_value=True)
    mocker.patch("src.services.connect._read_existing_tunnel_pid", return_value=7777)
    create_tunnel = mocker.patch("src.services.connect.create_tunnel")
    save_tunnel_pid = mocker.patch("src.services.connect.save_tunnel_pid")

    tunnel_pid, tunnel_reused = _ensure_tunnel(
        target=target,
        config=config,
        hostname="resolved.example.internal",
        username="ec2-user",
        keyfile="/tmp/test-key",
        port=2202,
        proxycmd="ssh -W %h:%p jump-host",
        internal_ip="10.0.0.10",
        local_port=16443,
    )

    assert tunnel_pid == 7777
    assert tunnel_reused is True
    create_tunnel.assert_not_called()
    save_tunnel_pid.assert_not_called()


def test_connect_cluster_returns_structured_result(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )

    mock_ssh_client = MagicMock()

    mocker.patch(
        "src.services.connect.detect_network_requirement",
        return_value=(None, None, False),
    )
    mocker.patch(
        "src.services.connect.load_ssh_config",
        return_value={"hostname": "acme-prod"},
    )
    mocker.patch(
        "src.services.connect.make_ssh_client",
        return_value=mock_ssh_client,
    )
    mocker.patch("src.services.connect.get_internal_ip", return_value="10.0.0.10")
    mocker.patch(
        "src.services.connect.fetch_remote_file_cached",
        return_value=("apiVersion: v1\nclusters: []\n", False),
    )
    mocker.patch(
        "src.services.connect.update_kubeconfig_server",
        return_value="apiVersion: v1\n",
    )
    mocker.patch("src.services.connect.merge_kubeconfig")
    mocker.patch("src.services.connect.get_unique_port", return_value=16443)
    mocker.patch("src.services.connect.create_tunnel", return_value=4242)
    mocker.patch("src.services.connect.save_tunnel_pid")
    mocker.patch("src.services.connect.save_network_metadata")

    result = connect_cluster(target=target, config=config)

    assert result.success is True
    assert result.context_name == "acme-prod"
    assert result.local_port == 16443
    assert result.internal_ip == "10.0.0.10"
    assert result.tunnel_pid == 4242
    assert result.used_cache is False
    assert result.network_requirement.type is None
    assert result.error is None
    mock_ssh_client.close.assert_called_once()


def test_connect_cluster_reuses_existing_active_tunnel(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )

    mock_ssh_client = MagicMock()
    pid_file = tmp_path / "acme-prod.pid"
    pid_file.write_text("7777")

    mocker.patch(
        "src.services.connect.detect_network_requirement",
        return_value=(None, None, False),
    )
    mocker.patch(
        "src.services.connect.load_ssh_config",
        return_value={"hostname": "acme-prod"},
    )
    mocker.patch(
        "src.services.connect.make_ssh_client",
        return_value=mock_ssh_client,
    )
    mocker.patch("src.services.connect.get_internal_ip", return_value="10.0.0.10")
    mocker.patch(
        "src.services.connect.fetch_remote_file_cached",
        return_value=("apiVersion: v1\nclusters: []\n", False),
    )
    mocker.patch(
        "src.services.connect.update_kubeconfig_server",
        return_value="apiVersion: v1\n",
    )
    mocker.patch("src.services.connect.merge_kubeconfig")
    mocker.patch("src.services.connect.get_unique_port", return_value=16443)
    mocker.patch("src.services.connect.is_tunnel_running", return_value=True)
    mocker.patch(
        "src.services.connect.get_tunnel_pid_file",
        return_value=pid_file,
    )
    create_tunnel_mock = mocker.patch("src.services.connect.create_tunnel")
    save_tunnel_pid_mock = mocker.patch("src.services.connect.save_tunnel_pid")
    mocker.patch("src.services.connect.save_network_metadata")

    result = connect_cluster(target=target, config=config)

    assert result.success is True
    assert result.context_name == "acme-prod"
    assert result.local_port == 16443
    assert result.internal_ip == "10.0.0.10"
    assert result.tunnel_pid == 7777
    assert result.error is None
    create_tunnel_mock.assert_not_called()
    save_tunnel_pid_mock.assert_not_called()
    mock_ssh_client.close.assert_called_once()


def test_connect_cluster_returns_error_when_create_tunnel_fails(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )

    mock_ssh_client = MagicMock()

    mocker.patch(
        "src.services.connect.detect_network_requirement",
        return_value=(None, None, False),
    )
    mocker.patch(
        "src.services.connect.load_ssh_config",
        return_value={"hostname": "acme-prod"},
    )
    mocker.patch(
        "src.services.connect.make_ssh_client",
        return_value=mock_ssh_client,
    )
    mocker.patch("src.services.connect.get_internal_ip", return_value="10.0.0.10")
    mocker.patch(
        "src.services.connect.fetch_remote_file_cached",
        return_value=("apiVersion: v1\nclusters: []\n", False),
    )
    mocker.patch(
        "src.services.connect.update_kubeconfig_server",
        return_value="apiVersion: v1\n",
    )
    mocker.patch("src.services.connect.merge_kubeconfig")
    mocker.patch("src.services.connect.get_unique_port", return_value=16443)
    mocker.patch("src.services.connect.is_tunnel_running", return_value=False)
    mocker.patch(
        "src.services.connect.create_tunnel",
        side_effect=RuntimeError("boom tunnel token=secret"),
    )

    result = connect_cluster(target=target, config=config)

    assert result.success is False
    assert result.context_name == "acme-prod"
    assert result.local_port is None
    assert result.internal_ip is None
    assert result.tunnel_pid is None
    assert result.error is not None
    assert result.error.code == "connect_failed"
    assert result.error.detail is None
    mock_ssh_client.close.assert_called_once()


def test_connect_cluster_returns_error_when_make_ssh_client_fails(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )

    mocker.patch(
        "src.services.connect.detect_network_requirement",
        return_value=(None, None, False),
    )
    mocker.patch(
        "src.services.connect.load_ssh_config",
        return_value={"hostname": "acme-prod"},
    )
    mocker.patch(
        "src.services.connect.make_ssh_client",
        side_effect=RuntimeError("boom ssh client private_key=/tmp/key"),
    )

    result = connect_cluster(target=target, config=config)

    assert result.success is False
    assert result.context_name == "acme-prod"
    assert result.local_port is None
    assert result.internal_ip is None
    assert result.tunnel_pid is None
    assert result.error is not None
    assert result.error.code == "connect_failed"
    assert result.error.detail is None


def test_connect_cluster_creates_tunnel_with_resolved_ssh_target(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="prod",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )

    mock_ssh_client = MagicMock()
    create_tunnel_mock = mocker.patch(
        "src.services.connect.create_tunnel",
        return_value=4242,
    )

    mocker.patch(
        "src.services.connect.detect_network_requirement",
        return_value=(None, None, False),
    )
    mocker.patch(
        "src.services.connect.load_ssh_config",
        return_value={"hostname": "ignored-by-resolver"},
    )
    mocker.patch(
        "src.services.connect.resolve_ssh_connection_target",
        return_value=(
            "resolved.example.internal",
            "ec2-user",
            "/tmp/test-key",
            2202,
            "ssh -W %h:%p jump-host",
        ),
    )
    mocker.patch(
        "src.services.connect.make_ssh_client",
        return_value=mock_ssh_client,
    )
    mocker.patch("src.services.connect.get_internal_ip", return_value="10.0.0.10")
    mocker.patch(
        "src.services.connect.fetch_remote_file_cached",
        return_value=("apiVersion: v1\nclusters: []\n", False),
    )
    mocker.patch(
        "src.services.connect.update_kubeconfig_server",
        return_value="apiVersion: v1\n",
    )
    mocker.patch("src.services.connect.merge_kubeconfig")
    mocker.patch("src.services.connect.get_unique_port", return_value=16443)
    mocker.patch("src.services.connect.save_tunnel_pid")
    mocker.patch("src.services.connect.save_network_metadata")

    result = connect_cluster(target=target, config=config)

    assert result.success is True
    create_tunnel_mock.assert_called_once_with(
        "resolved.example.internal",
        "10.0.0.10",
        16443,
        6443,
        username="ec2-user",
        key_filename="/tmp/test-key",
        port=2202,
        proxycmd="ssh -W %h:%p jump-host",
    )
    mock_ssh_client.close.assert_called_once()


def test_connect_cluster_fails_fast_for_vpn_or_sshuttle_targets(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="vpn",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )

    mocker.patch(
        "src.services.connect.detect_network_requirement",
        return_value=("sshuttle", "10.0.0.0/24", False),
    )

    result = connect_cluster(
        target=target,
        config=config,
        allow_manual_network=False,
    )

    assert result.success is False
    assert result.context_name == "acme-vpn"
    assert result.error is not None
    assert result.error.code == "network_requirement_unmet"
    assert result.network_requirement.type == "sshuttle"
    assert result.network_requirement.network_range == "10.0.0.0/24"


def test_connect_cluster_fails_fast_when_vpn_required_without_network_type(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="vpn",
        group="k3s_cluster",
        host_config={"ansible_host": "10.0.0.10"},
    )

    load_ssh_config_mock = mocker.patch("src.services.connect.load_ssh_config")
    make_ssh_client_mock = mocker.patch("src.services.connect.make_ssh_client")
    mocker.patch(
        "src.services.connect.detect_network_requirement",
        return_value=(None, None, True),
    )

    result = connect_cluster(
        target=target,
        config=config,
        allow_manual_network=False,
    )

    assert result.success is False
    assert result.context_name == "acme-vpn"
    assert result.error is not None
    assert result.error.code == "network_requirement_unmet"
    assert result.network_requirement.type is None
    assert result.network_requirement.network_range is None
    assert result.network_requirement.needs_vpn is True
    load_ssh_config_mock.assert_not_called()
    make_ssh_client_mock.assert_not_called()


def test_connect_cluster_detects_vpn_requirement_from_group_vars(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    target = ClusterTarget(
        company="acme",
        host_alias="vpn",
        group="k3s_cluster",
        host_config={"ansible_host": "8.8.8.8"},
        group_vars={"argocd_use_socks5_proxy": True},
    )

    load_ssh_config_mock = mocker.patch("src.services.connect.load_ssh_config")
    make_ssh_client_mock = mocker.patch("src.services.connect.make_ssh_client")

    result = connect_cluster(
        target=target,
        config=config,
        allow_manual_network=False,
    )

    assert result.success is False
    assert result.context_name == "acme-vpn"
    assert result.error is not None
    assert result.error.code == "network_requirement_unmet"
    assert result.network_requirement.type is None
    assert result.network_requirement.network_range is None
    assert result.network_requirement.needs_vpn is True
    load_ssh_config_mock.assert_not_called()
    make_ssh_client_mock.assert_not_called()


def test_connect_multiple_delegates_to_connect_cluster(
    mocker: MockerFixture,
    tmp_path: Path,
) -> None:
    config = build_config(tmp_path)
    targets = [
        ClusterTarget(company="acme", host_alias="one", group="k3s_cluster"),
        ClusterTarget(company="acme", host_alias="two", group="k3s_cluster"),
    ]
    first = MagicMock()
    second = MagicMock()

    connect_mock = mocker.patch(
        "src.services.connect.connect_cluster",
        side_effect=[first, second],
    )

    results = connect_multiple(targets=targets, config=config, allow_manual_network=False)

    assert results == [first, second]
    assert connect_mock.call_count == 2
