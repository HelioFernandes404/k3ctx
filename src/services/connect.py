"""Shared cluster connection orchestration."""

from __future__ import annotations

from pathlib import Path

from src.kubeconfig import merge_kubeconfig, update_kubeconfig_server
from src.logging_config import get_logger
from src.models import (
    ClusterTarget,
    ConnectResult,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)
from src.network import detect_network_requirement
from src.ssh import (
    fetch_remote_file_cached,
    get_internal_ip,
    load_ssh_config,
    make_ssh_client,
    resolve_ssh_connection_target,
)
from src.tunnel import (
    build_sshuttle_command,
    create_tunnel,
    get_tunnel_pid_file,
    get_unique_port,
    is_tunnel_running,
    save_network_metadata,
    save_tunnel_pid,
)

logger = get_logger()


def _read_existing_tunnel_pid(context_name: str) -> int | None:
    pid_file = get_tunnel_pid_file(context_name)
    if not pid_file.exists():
        return None

    try:
        return int(pid_file.read_text().strip())
    except ValueError:
        return None


def _build_public_connect_error() -> OperationError:
    return OperationError(
        code="connect_failed",
        message="Cluster connection failed",
    )


def _resolve_network_requirement(
    target: ClusterTarget,
    *,
    allow_manual_network: bool,
) -> tuple[NetworkRequirement, OperationError | None]:
    network_type, network_range, needs_vpn = detect_network_requirement(
        target.host_config,
        target.group_vars,
        target.group,
    )
    requirement = NetworkRequirement(
        type=network_type,
        network_range=network_range,
        needs_vpn=needs_vpn,
    )

    if allow_manual_network or (not needs_vpn and network_type != "sshuttle"):
        return requirement, None

    logger.warning(
        "Cluster connection blocked by network requirement",
        extra={
            "event": "connect.network_requirement_unmet",
            "context_name": target.context_name,
            "network_type": network_type,
            "needs_vpn": needs_vpn,
        },
    )
    return requirement, OperationError(
        code="network_requirement_unmet",
        message="Cluster requires manual network setup before connection",
    )


def _prepare_local_kubeconfig(
    target: ClusterTarget,
    config: EffectiveConfig,
    ssh_client: object,
) -> tuple[str, int, bool]:
    internal_ip = get_internal_ip(ssh_client)
    cache_path = Path.home() / ".cache" / "k9s-config" / f"{target.context_name}.yml"
    content, used_cache = fetch_remote_file_cached(
        ssh_client,
        config.remote_k3s_config_path,
        cache_path,
    )
    local_port = get_unique_port(
        target.context_name,
        config.port_range_start,
        config.port_range_size,
    )
    updated_content = update_kubeconfig_server(
        content,
        internal_ip,
        config.k3s_api_port,
        use_localhost=True,
        local_port=local_port,
    )
    merge_kubeconfig(updated_content, target.context_name)
    return internal_ip, local_port, used_cache


def _ensure_tunnel(
    *,
    target: ClusterTarget,
    config: EffectiveConfig,
    hostname: str,
    username: str,
    keyfile: str | None,
    port: int,
    proxycmd: str | None,
    internal_ip: str,
    local_port: int,
) -> tuple[int | None, bool]:
    if is_tunnel_running(target.context_name):
        tunnel_pid = _read_existing_tunnel_pid(target.context_name)
        logger.info(
            "Reusing active tunnel",
            extra={
                "event": "connect.tunnel_reused",
                "context_name": target.context_name,
                "tunnel_pid": tunnel_pid,
            },
        )
        return tunnel_pid, True

    tunnel_pid = create_tunnel(
        hostname,
        internal_ip,
        local_port,
        config.k3s_api_port,
        username=username,
        key_filename=keyfile,
        port=port,
        proxycmd=proxycmd,
    )
    save_tunnel_pid(target.context_name, tunnel_pid)
    return tunnel_pid, False


def connect_cluster(
    target: ClusterTarget,
    config: EffectiveConfig,
    *,
    allow_manual_network: bool = True,
) -> ConnectResult:
    logger.info(
        "Starting cluster connection",
        extra={
            "event": "connect.started",
            "context_name": target.context_name,
            "host_alias": target.host_alias,
            "allow_manual_network": allow_manual_network,
        },
    )
    requirement, requirement_error = _resolve_network_requirement(
        target,
        allow_manual_network=allow_manual_network,
    )
    if requirement_error is not None:
        return ConnectResult(
            success=False,
            context_name=target.context_name,
            local_port=None,
            internal_ip=None,
            tunnel_pid=None,
            used_cache=False,
            network_requirement=requirement,
            error=requirement_error,
        )

    ssh_client = None
    try:
        ssh_config = load_ssh_config(target.host_alias, config.ssh_config_path)
        hostname, username, keyfile, port, proxycmd = resolve_ssh_connection_target(
            target.host_alias,
            ssh_config,
            ssh_key_path=config.ssh_key_path,
            host_config=target.host_config,
        )
        ssh_client = make_ssh_client(hostname, username, keyfile, port, proxycmd)
        internal_ip, local_port, used_cache = _prepare_local_kubeconfig(
            target,
            config,
            ssh_client,
        )
        tunnel_pid, tunnel_reused = _ensure_tunnel(
            target=target,
            config=config,
            hostname=hostname,
            username=username,
            keyfile=keyfile,
            port=port,
            proxycmd=proxycmd,
            internal_ip=internal_ip,
            local_port=local_port,
        )
        save_network_metadata(
            context_name=target.context_name,
            network_type=requirement.type,
            network_range=requirement.network_range,
            sshuttle_command=build_sshuttle_command(
                requirement.type,
                requirement.network_range,
            ),
            needs_vpn=requirement.needs_vpn,
            internal_ip=internal_ip,
        )
        logger.info(
            "Cluster connection completed",
            extra={
                "event": "connect.finished",
                "context_name": target.context_name,
                "used_cache": used_cache,
                "tunnel_reused": tunnel_reused,
                "network_type": requirement.type,
                "needs_vpn": requirement.needs_vpn,
            },
        )
        return ConnectResult(
            success=True,
            context_name=target.context_name,
            local_port=local_port,
            internal_ip=internal_ip,
            tunnel_pid=tunnel_pid,
            used_cache=used_cache,
            network_requirement=requirement,
        )
    except Exception as exc:
        logger.error(
            "Cluster connection failed",
            extra={
                "event": "connect.failed",
                "context_name": target.context_name,
                "error_type": type(exc).__name__,
            },
        )
        return ConnectResult(
            success=False,
            context_name=target.context_name,
            local_port=None,
            internal_ip=None,
            tunnel_pid=None,
            used_cache=False,
            network_requirement=requirement,
            error=_build_public_connect_error(),
        )
    finally:
        if ssh_client is not None:
            ssh_client.close()


def connect_multiple(
    targets: list[ClusterTarget],
    config: EffectiveConfig,
    *,
    allow_manual_network: bool = True,
) -> list[ConnectResult]:
    logger.info(
        "Starting multi-cluster connection",
        extra={
            "event": "connect_multiple.started",
            "target_count": len(targets),
            "allow_manual_network": allow_manual_network,
        },
    )
    results = [
        connect_cluster(
            target=target,
            config=config,
            allow_manual_network=allow_manual_network,
        )
        for target in targets
    ]
    logger.info(
        "Completed multi-cluster connection",
        extra={
            "event": "connect_multiple.finished",
            "target_count": len(targets),
            "success_count": sum(1 for result in results if result.success),
        },
    )
    return results
