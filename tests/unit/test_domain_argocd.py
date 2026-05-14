"""Unit tests for the ArgoCD domain model."""

from __future__ import annotations

import pytest

from src.domain.argocd import ArgocdConfig


class TestArgocdConfigDefaults:
    def test_disabled_factory(self) -> None:
        cfg = ArgocdConfig.disabled()
        assert cfg.enabled is False
        assert cfg.namespace == "argocd"
        assert cfg.node_port is None

    def test_enabled_with_defaults(self) -> None:
        cfg = ArgocdConfig(enabled=True)
        assert cfg.namespace == "argocd"
        assert cfg.node_port is None

    def test_is_frozen(self) -> None:
        cfg = ArgocdConfig(enabled=True)
        with pytest.raises(Exception):
            cfg.enabled = False  # type: ignore[misc]


class TestArgocdConfigFromHostConfig:
    def test_returns_disabled_when_not_configured(self) -> None:
        cfg = ArgocdConfig.from_host_config({})
        assert cfg.enabled is False

    def test_returns_disabled_when_flag_false(self) -> None:
        cfg = ArgocdConfig.from_host_config({"argocd_enabled": False})
        assert cfg.enabled is False

    def test_reads_enabled_from_host_config(self) -> None:
        cfg = ArgocdConfig.from_host_config({"argocd_enabled": True})
        assert cfg.enabled is True

    def test_reads_namespace_from_host_config(self) -> None:
        cfg = ArgocdConfig.from_host_config(
            {"argocd_enabled": True, "argocd_namespace": "argocd-system"}
        )
        assert cfg.namespace == "argocd-system"

    def test_reads_node_port_from_host_config(self) -> None:
        cfg = ArgocdConfig.from_host_config(
            {"argocd_enabled": True, "argocd_node_port": 30080}
        )
        assert cfg.node_port == 30080

    def test_host_config_takes_precedence_over_group_vars(self) -> None:
        cfg = ArgocdConfig.from_host_config(
            {"argocd_enabled": True, "argocd_namespace": "host-ns"},
            group_vars={"argocd_namespace": "group-ns"},
        )
        assert cfg.namespace == "host-ns"

    def test_falls_back_to_group_vars_when_key_missing_in_host(self) -> None:
        cfg = ArgocdConfig.from_host_config(
            {"argocd_enabled": True},
            group_vars={"argocd_node_port": 31443},
        )
        assert cfg.node_port == 31443

    def test_reads_enabled_from_group_vars_when_absent_in_host(self) -> None:
        cfg = ArgocdConfig.from_host_config(
            {},
            group_vars={"argocd_enabled": True, "argocd_node_port": 30080},
        )
        assert cfg.enabled is True
        assert cfg.node_port == 30080

    def test_default_namespace_when_not_specified(self) -> None:
        cfg = ArgocdConfig.from_host_config({"argocd_enabled": True})
        assert cfg.namespace == "argocd"

    def test_node_port_none_when_not_specified(self) -> None:
        cfg = ArgocdConfig.from_host_config({"argocd_enabled": True})
        assert cfg.node_port is None

    def test_plaintext_defaults_to_false(self) -> None:
        cfg = ArgocdConfig.from_host_config({"argocd_enabled": True})
        assert cfg.plaintext is False

    def test_reads_plaintext_true_from_host_config(self) -> None:
        cfg = ArgocdConfig.from_host_config(
            {"argocd_enabled": True, "argocd_plaintext": True}
        )
        assert cfg.plaintext is True

    def test_reads_plaintext_from_group_vars(self) -> None:
        cfg = ArgocdConfig.from_host_config(
            {"argocd_enabled": True},
            group_vars={"argocd_plaintext": True},
        )
        assert cfg.plaintext is True
