"""Local infrastructure adapter for connecting to K3s clusters."""

from __future__ import annotations

import base64
import os
import ssl
import tempfile
import time
import urllib.error
import urllib.request
from contextlib import ExitStack

import yaml

from src.application.ports import ClusterConnectionError, ConnectionArtifacts
from src.app_paths import get_kubeconfig_cache_path
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
    kill_tunnel,
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
) -> tuple[str, int, bool, str]:
    internal_ip = get_internal_ip(ssh_client)
    cache_path = get_kubeconfig_cache_path(target.context_name)
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
    return internal_ip, local_port, used_cache, updated_content


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


def _wait_for_kubernetes_api(
    local_port: int,
    *,
    kubeconfig_text: str,
    timeout_seconds: float,
    poll_interval_seconds: float,
) -> None:
    url = f"https://127.0.0.1:{local_port}/version"
    deadline = time.monotonic() + timeout_seconds
    last_error: Exception | None = None

    kubeconfig = yaml.safe_load(kubeconfig_text) or {}
    cluster = ((kubeconfig.get("clusters") or [{}])[0]).get("cluster") or {}
    user = ((kubeconfig.get("users") or [{}])[0]).get("user") or {}
    ca_data = cluster.get("certificate-authority-data")
    client_cert_data = user.get("client-certificate-data")
    client_key_data = user.get("client-key-data")
    token = user.get("token")

    with ExitStack() as stack:
        if isinstance(ca_data, str) and ca_data:
            ssl_context = ssl.create_default_context(
                cadata=base64.b64decode(ca_data).decode("utf-8")
            )
            ssl_context.check_hostname = False
        else:
            ssl_context = ssl.create_default_context()
            ssl_context.check_hostname = False
            ssl_context.verify_mode = ssl.CERT_NONE

        if (
            isinstance(client_cert_data, str)
            and client_cert_data
            and isinstance(client_key_data, str)
            and client_key_data
        ):
            cert_file = tempfile.NamedTemporaryFile(mode="w", delete=False)
            key_file = tempfile.NamedTemporaryFile(mode="w", delete=False)
            stack.callback(os.unlink, cert_file.name)
            stack.callback(os.unlink, key_file.name)
            cert_file.write(base64.b64decode(client_cert_data).decode("utf-8"))
            key_file.write(base64.b64decode(client_key_data).decode("utf-8"))
            cert_file.close()
            key_file.close()
            ssl_context.load_cert_chain(cert_file.name, key_file.name)

        request = urllib.request.Request(url)
        if isinstance(token, str) and token:
            request.add_header("Authorization", f"Bearer {token}")

        while time.monotonic() < deadline:
            remaining = max(0.1, deadline - time.monotonic())
            request_timeout = min(max(poll_interval_seconds, 2.0), remaining)
            try:
                with urllib.request.urlopen(
                    request,
                    timeout=request_timeout,
                    context=ssl_context,
                ) as response:
                    if 200 <= response.status < 500:
                        return
            except urllib.error.HTTPError as exc:
                if exc.code < 500:
                    return
                last_error = exc
            except Exception as exc:  # pragma: no cover - exercised through public failure path
                last_error = exc
            time.sleep(min(poll_interval_seconds, max(0.0, deadline - time.monotonic())))

    detail = None if last_error is None else str(last_error)
    raise ClusterConnectionError(
        code="kubernetes_api_unreachable",
        message=f"Kubernetes API did not become ready on {url}",
        detail=detail,
        retryable=True,
    )


def _verify_or_recreate_tunnel(
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
    kubeconfig_text: str,
    tunnel_pid: int | None,
    tunnel_reused: bool,
    timeout_seconds: float,
    poll_interval_seconds: float,
) -> tuple[int | None, bool]:
    try:
        _wait_for_kubernetes_api(
            local_port,
            kubeconfig_text=kubeconfig_text,
            timeout_seconds=timeout_seconds,
            poll_interval_seconds=poll_interval_seconds,
        )
        return tunnel_pid, tunnel_reused
    except ClusterConnectionError:
        if not tunnel_reused:
            kill_tunnel(target.context_name)
            raise

        kill_tunnel(target.context_name)
        replacement_pid, replacement_reused = _ensure_tunnel(
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
        try:
            _wait_for_kubernetes_api(
                local_port,
                kubeconfig_text=kubeconfig_text,
                timeout_seconds=timeout_seconds,
                poll_interval_seconds=poll_interval_seconds,
            )
        except ClusterConnectionError:
            if not replacement_reused:
                kill_tunnel(target.context_name)
            raise
        return replacement_pid, replacement_reused


class LocalClusterConnector:
    def __init__(
        self,
        *,
        verify_api_readiness: bool | None = None,
        api_ready_timeout_seconds: float | None = None,
        api_ready_interval_seconds: float = 0.25,
    ) -> None:
        if verify_api_readiness is None:
            verify_api_readiness = (
                os.getenv("K9S_VERIFY_API_READY", "1").strip().lower()
                not in {"0", "false", "no", "off"}
            )
        if api_ready_timeout_seconds is None:
            api_ready_timeout_seconds = float(
                os.getenv("K9S_API_READY_TIMEOUT_SECONDS", "15.0")
            )

        self.verify_api_readiness = verify_api_readiness
        self.api_ready_timeout_seconds = api_ready_timeout_seconds
        self.api_ready_interval_seconds = api_ready_interval_seconds

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
            internal_ip, local_port, used_cache, updated_content = _prepare_local_kubeconfig(
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
            if self.verify_api_readiness:
                tunnel_pid, tunnel_reused = _verify_or_recreate_tunnel(
                    target=target,
                    config=config,
                    hostname=hostname,
                    username=username,
                    keyfile=keyfile,
                    port=port,
                    proxycmd=proxycmd,
                    internal_ip=internal_ip,
                    local_port=local_port,
                    kubeconfig_text=updated_content,
                    tunnel_pid=tunnel_pid,
                    tunnel_reused=tunnel_reused,
                    timeout_seconds=self.api_ready_timeout_seconds,
                    poll_interval_seconds=self.api_ready_interval_seconds,
                )

            merge_kubeconfig(updated_content, target.context_name)
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
