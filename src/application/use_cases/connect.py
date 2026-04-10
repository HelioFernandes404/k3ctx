"""Interface-agnostic use cases for cluster connection flows."""

from __future__ import annotations

from src.application.ports import ClusterConnector
from src.domain.models import (
    ClusterTarget,
    ConnectResult,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)
from src.domain.network import detect_network_requirement
from src.logging_config import get_logger

logger = get_logger()


def build_network_requirement(target: ClusterTarget) -> NetworkRequirement:
    network_type, network_range, needs_vpn = detect_network_requirement(
        target.host_config,
        target.group_vars,
        target.group,
    )
    return NetworkRequirement(
        type=network_type,
        network_range=network_range,
        needs_vpn=needs_vpn,
    )


def _build_public_connect_error() -> OperationError:
    return OperationError(
        code="connect_failed",
        message="Cluster connection failed",
    )


def connect_cluster(
    target: ClusterTarget,
    config: EffectiveConfig,
    connector: ClusterConnector,
    *,
    allow_manual_network: bool = True,
) -> ConnectResult:
    logger.info(
        "Starting cluster connection use case",
        extra={
            "event": "application.connect.started",
            "context_name": target.context_name,
            "allow_manual_network": allow_manual_network,
        },
    )
    requirement = build_network_requirement(target)

    if not allow_manual_network and (
        requirement.needs_vpn or requirement.type == "sshuttle"
    ):
        logger.warning(
            "Cluster connection blocked by manual network requirement",
            extra={
                "event": "application.connect.network_requirement_unmet",
                "context_name": target.context_name,
                "network_type": requirement.type,
                "needs_vpn": requirement.needs_vpn,
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
            error=OperationError(
                code="network_requirement_unmet",
                message="Cluster requires manual network setup before connection",
            ),
        )

    try:
        artifacts = connector.connect(target, config, requirement)
    except Exception as exc:
        logger.error(
            "Cluster connection use case failed",
            extra={
                "event": "application.connect.failed",
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

    logger.info(
        "Cluster connection use case completed",
        extra={
            "event": "application.connect.finished",
            "context_name": target.context_name,
            "local_port": artifacts.local_port,
        },
    )
    return ConnectResult(
        success=True,
        context_name=target.context_name,
        local_port=artifacts.local_port,
        internal_ip=artifacts.internal_ip,
        tunnel_pid=artifacts.tunnel_pid,
        used_cache=artifacts.used_cache,
        network_requirement=requirement,
    )


def connect_multiple(
    targets: list[ClusterTarget],
    config: EffectiveConfig,
    connector: ClusterConnector,
    *,
    allow_manual_network: bool = True,
) -> list[ConnectResult]:
    logger.info(
        "Starting multi-cluster connection use case",
        extra={
            "event": "application.connect_multiple.started",
            "target_count": len(targets),
        },
    )
    results = [
        connect_cluster(
            target=target,
            config=config,
            connector=connector,
            allow_manual_network=allow_manual_network,
        )
        for target in targets
    ]
    logger.info(
        "Completed multi-cluster connection use case",
        extra={
            "event": "application.connect_multiple.finished",
            "target_count": len(targets),
            "success_count": sum(1 for result in results if result.success),
        },
    )
    return results
