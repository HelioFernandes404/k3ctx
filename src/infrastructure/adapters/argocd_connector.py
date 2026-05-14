"""Local infrastructure adapter for ArgoCD CLI integration."""

from __future__ import annotations

import base64
import shutil
import subprocess
from typing import Optional

from src.application.ports import ArgocdLoginResult
from src.domain.argocd import ArgocdConfig
from src.logging_config import get_logger
from src.tunnel import (
    create_tunnel,
    get_unique_port,
    is_tunnel_running,
    save_tunnel_pid,
)

logger = get_logger()

_ARGOCD_PORT_RANGE_START = 28000
_ARGOCD_PORT_RANGE_SIZE = 10000


def _fetch_argocd_password(context_name: str, namespace: str) -> Optional[str]:
    try:
        result = subprocess.run(
            [
                "kubectl", "get", "secret", "argocd-initial-admin-secret",
                "-n", namespace,
                "-o", "jsonpath={.data.password}",
                "--context", context_name,
            ],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode != 0 or not result.stdout.strip():
            return None
        return base64.b64decode(result.stdout.strip()).decode("utf-8")
    except Exception:
        return None


class LocalArgocdConnector:
    def setup(
        self,
        context_name: str,
        argocd_config: ArgocdConfig,
        *,
        hostname: str,
        username: str,
        keyfile: Optional[str],
        port: int,
        proxycmd: Optional[str],
        internal_ip: str,
    ) -> ArgocdLoginResult:
        if not argocd_config.enabled or argocd_config.node_port is None:
            return ArgocdLoginResult(
                success=False,
                local_port=None,
                skipped=True,
                message="ArgoCD not configured for this cluster",
            )

        argocd_context = f"{context_name}-argocd"
        local_port = get_unique_port(
            argocd_context,
            _ARGOCD_PORT_RANGE_START,
            _ARGOCD_PORT_RANGE_SIZE,
        )

        if not is_tunnel_running(argocd_context):
            tunnel_pid = create_tunnel(
                hostname,
                internal_ip,
                local_port,
                argocd_config.node_port,
                username=username,
                key_filename=keyfile,
                port=port,
                proxycmd=proxycmd,
            )
            save_tunnel_pid(argocd_context, tunnel_pid)
            logger.info(
                "Opened ArgoCD SSH tunnel",
                extra={
                    "event": "infrastructure.argocd.tunnel_opened",
                    "context_name": context_name,
                    "local_port": local_port,
                    "node_port": argocd_config.node_port,
                },
            )
        else:
            logger.info(
                "Reusing active ArgoCD tunnel",
                extra={
                    "event": "infrastructure.argocd.tunnel_reused",
                    "context_name": context_name,
                    "local_port": local_port,
                },
            )

        if not shutil.which("argocd"):
            return ArgocdLoginResult(
                success=False,
                local_port=local_port,
                message=f"argocd CLI not found in PATH; tunnel open at 127.0.0.1:{local_port}",
            )

        password = _fetch_argocd_password(context_name, argocd_config.namespace)
        if password is None:
            return ArgocdLoginResult(
                success=False,
                local_port=local_port,
                message=(
                    f"argocd-initial-admin-secret not found in namespace '{argocd_config.namespace}'; "
                    f"run: argocd login --insecure 127.0.0.1:{local_port}"
                ),
            )

        tls_flag = "--plaintext" if argocd_config.plaintext else "--insecure"
        try:
            subprocess.run(
                [
                    "argocd", "login", tls_flag,
                    "--username", "admin",
                    "--password", password,
                    f"127.0.0.1:{local_port}",
                ],
                capture_output=True,
                text=True,
                timeout=30,
                check=True,
            )
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as exc:
            return ArgocdLoginResult(
                success=False,
                local_port=local_port,
                message=f"argocd login failed: {exc}",
            )

        logger.info(
            "ArgoCD CLI login successful",
            extra={
                "event": "infrastructure.argocd.login_success",
                "context_name": context_name,
                "local_port": local_port,
            },
        )
        return ArgocdLoginResult(success=True, local_port=local_port)
