"""Local infrastructure adapter for connecting to K3s clusters."""

from __future__ import annotations

from pathlib import Path

from src.application.ports import ConnectionArtifacts
from src.domain.models import ClusterTarget, EffectiveConfig, NetworkRequirement
from src.kubeconfig import merge_kubeconfig, update_kubeconfig_server
from src.logging_config import get_logger
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
                "event": "infrastructure.connector.tunnel_reused",
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


class LocalClusterConnector:
    def connect(
        self,
        target: ClusterTarget,
        config: EffectiveConfig,
        requirement: NetworkRequirement,
    ) -> ConnectionArtifacts:
        logger.info(
            "Starting local cluster connection adapter",
            extra={
                "event": "infrastructure.connector.started",
                "context_name": target.context_name,
                "host_alias": target.host_alias,
            },
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
                "Completed local cluster connection adapter",
                extra={
                    "event": "infrastructure.connector.finished",
                    "context_name": target.context_name,
                    "used_cache": used_cache,
                    "tunnel_reused": tunnel_reused,
                },
            )
            return ConnectionArtifacts(
                local_port=local_port,
                internal_ip=internal_ip,
                tunnel_pid=tunnel_pid,
                used_cache=used_cache,
            )
        except Exception:
            logger.error(
                "Local cluster connection adapter failed",
                extra={
                    "event": "infrastructure.connector.failed",
                    "context_name": target.context_name,
                },
            )
            raise
        finally:
            if ssh_client is not None:
                ssh_client.close()
