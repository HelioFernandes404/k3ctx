# MCP Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extrair um core reutilizavel para conexao e status de clusters e adicionar um servidor MCP com FastMCP sem quebrar `make run`, `make multi-connect`, `make status` e `make k9s`.

**Architecture:** A logica de negocio sai dos scripts e passa para `src/services/`, apoiada por modelos tipados em `src/models.py`. A camada manual continua interativa em `fetch_k3s_config.py`, `multi_connect.py` e `src/cli.py`, enquanto `src/mcp_server.py` usa FastMCP apenas como interface, com tools pequenas, tipadas, com logs estruturados e erros controlados.

**Tech Stack:** Python 3.10+, pytest, mypy, questionary, paramiko, PyYAML, FastMCP.

---

## File Structure

- Create: `src/models.py`
- Create: `src/services/__init__.py`
- Create: `src/services/connect.py`
- Create: `src/services/status.py`
- Create: `src/services/contexts.py`
- Create: `src/services/inventory_service.py`
- Create: `src/mcp_server.py`
- Create: `tests/unit/test_models.py`
- Create: `tests/unit/test_services_connect.py`
- Create: `tests/unit/test_services_status.py`
- Create: `tests/unit/test_services_contexts.py`
- Create: `tests/unit/test_services_inventory_service.py`
- Create: `tests/unit/test_mcp_server.py`
- Modify: `fetch_k3s_config.py`
- Modify: `multi_connect.py`
- Modify: `src/config.py`
- Modify: `src/inventory.py`
- Modify: `src/network.py`
- Modify: `src/network_validator.py`
- Modify: `src/ssh.py`
- Modify: `src/kubeconfig.py`
- Modify: `src/tunnel.py`
- Modify: `src/multi_status.py`
- Modify: `src/logging_config.py`
- Modify: `pyproject.toml`
- Modify: `Makefile`
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `tests/unit/test_fetch_k3s_config.py`
- Modify: `tests/unit/test_cli.py`
- Modify: `tests/unit/test_config.py`
- Modify: `tests/unit/test_inventory.py`
- Modify: `tests/unit/test_kubeconfig.py`
- Modify: `tests/unit/test_tunnel.py`
- Modify: `tests/unit/test_ssh.py`
- Modify: `tests/unit/test_network.py`
- Modify: `tests/unit/test_logging_config.py`
- Modify: `tests/smoke/test_e2e.py`

### Task 1: Introduce Typed Core Models

**Files:**
- Create: `src/models.py`
- Test: `tests/unit/test_models.py`

- [ ] **Step 1: Write the failing tests for the typed contracts**

```python
from src.models import (
    ClusterTarget,
    ConnectResult,
    EffectiveConfig,
    NetworkRequirement,
    OperationError,
)


def test_effective_config_normalizes_paths_and_ports(tmp_path):
    config = EffectiveConfig(
        inventory_path=tmp_path / "inventory",
        ssh_config_path="~/.ssh/config",
        ssh_key_path="~/.ssh/id_ed25519",
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )

    assert config.k3s_api_port == 6443
    assert config.port_range_start == 16443


def test_connect_result_exposes_safe_summary():
    result = ConnectResult(
        success=True,
        context_name="acme-prod",
        local_port=16443,
        internal_ip="10.0.0.10",
        tunnel_pid=4242,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
    )

    assert result.to_public_dict()["context_name"] == "acme-prod"
    assert "internal_ip" in result.to_public_dict()


def test_operation_error_redacts_sensitive_details():
    error = OperationError(
        code="ssh_connect_failed",
        message="SSH connection failed",
        detail="token=secret",
        retryable=True,
    )

    public = error.to_public_dict()
    assert public["code"] == "ssh_connect_failed"
    assert "secret" not in str(public)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_models.py -q`
Expected: FAIL with `ModuleNotFoundError` or missing symbols from `src.models`

- [ ] **Step 3: Write the minimal implementation**

```python
from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Optional


@dataclass(frozen=True)
class EffectiveConfig:
    inventory_path: Path
    ssh_config_path: str
    ssh_key_path: str
    remote_k3s_config_path: str
    k3s_api_port: int
    port_range_start: int
    port_range_size: int


@dataclass(frozen=True)
class NetworkRequirement:
    type: Optional[str]
    network_range: Optional[str]
    needs_vpn: bool = False

    @classmethod
    def none(cls) -> "NetworkRequirement":
        return cls(type=None, network_range=None, needs_vpn=False)


@dataclass(frozen=True)
class ClusterTarget:
    company: str
    host_alias: str
    group: str
    host_config: dict[str, Any] = field(default_factory=dict)

    @property
    def context_name(self) -> str:
        return f"{self.company}-{self.host_alias}"


@dataclass(frozen=True)
class OperationError:
    code: str
    message: str
    detail: Optional[str] = None
    retryable: bool = False

    def to_public_dict(self) -> dict[str, Any]:
        return {"code": self.code, "message": self.message, "retryable": self.retryable}


@dataclass(frozen=True)
class ConnectResult:
    success: bool
    context_name: str
    local_port: Optional[int]
    internal_ip: Optional[str]
    tunnel_pid: Optional[int]
    used_cache: bool
    network_requirement: NetworkRequirement
    error: Optional[OperationError] = None

    def to_public_dict(self) -> dict[str, Any]:
        return {
            "success": self.success,
            "context_name": self.context_name,
            "local_port": self.local_port,
            "internal_ip": self.internal_ip,
            "tunnel_pid": self.tunnel_pid,
            "used_cache": self.used_cache,
            "network_requirement": {
                "type": self.network_requirement.type,
                "network_range": self.network_requirement.network_range,
                "needs_vpn": self.network_requirement.needs_vpn,
            },
            "error": None if self.error is None else self.error.to_public_dict(),
        }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_models.py -q`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/models.py tests/unit/test_models.py
git commit -m "feat: add typed core models"
```

### Task 2: Extract the Connect Service

**Files:**
- Create: `src/services/__init__.py`
- Create: `src/services/connect.py`
- Modify: `src/network.py`
- Modify: `src/ssh.py`
- Modify: `src/kubeconfig.py`
- Modify: `src/tunnel.py`
- Test: `tests/unit/test_services_connect.py`
- Modify: `tests/unit/test_fetch_k3s_config.py`

- [ ] **Step 1: Write the failing tests for single-cluster connection orchestration**

```python
from unittest.mock import MagicMock

from src.models import ClusterTarget, EffectiveConfig
from src.services.connect import connect_cluster


def test_connect_cluster_returns_structured_result(mocker, tmp_path):
    config = EffectiveConfig(
        inventory_path=tmp_path / "inventory",
        ssh_config_path="~/.ssh/config",
        ssh_key_path="~/.ssh/id_ed25519",
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )
    target = ClusterTarget(company="acme", host_alias="prod", group="k3s_cluster")

    mocker.patch("src.services.connect.load_ssh_config", return_value={"hostname": "acme-prod"})
    mocker.patch("src.services.connect.make_ssh_client", return_value=MagicMock())
    mocker.patch("src.services.connect.get_internal_ip", return_value="10.0.0.10")
    mocker.patch("src.services.connect.fetch_remote_file_cached", return_value=("apiVersion: v1\nclusters: []\n", False))
    mocker.patch("src.services.connect.update_kubeconfig_server", return_value="apiVersion: v1\n")
    mocker.patch("src.services.connect.merge_kubeconfig")
    mocker.patch("src.services.connect.create_tunnel", return_value=4242)
    mocker.patch("src.services.connect.save_tunnel_pid")

    result = connect_cluster(target=target, config=config)

    assert result.success is True
    assert result.context_name == "acme-prod"
    assert result.tunnel_pid == 4242


def test_connect_cluster_fails_fast_for_vpn_or_sshuttle_targets(mocker, tmp_path):
    config = EffectiveConfig(
        inventory_path=tmp_path / "inventory",
        ssh_config_path="~/.ssh/config",
        ssh_key_path="~/.ssh/id_ed25519",
        remote_k3s_config_path="/etc/rancher/k3s/k3s.yaml",
        k3s_api_port=6443,
        port_range_start=16443,
        port_range_size=10000,
    )
    target = ClusterTarget(company="acme", host_alias="vpn", group="k3s_cluster")

    mocker.patch("src.services.connect.detect_network_requirement", return_value=("sshuttle", "10.0.0.0/24", False))

    result = connect_cluster(target=target, config=config, allow_manual_network=False)

    assert result.success is False
    assert result.error.code == "network_requirement_unmet"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_services_connect.py tests/unit/test_fetch_k3s_config.py -q`
Expected: FAIL with missing module `src.services.connect` and outdated `fetch_k3s_config` contract

- [ ] **Step 3: Write the minimal implementation**

```python
from __future__ import annotations

from pathlib import Path
from typing import Optional

from src.kubeconfig import merge_kubeconfig, update_kubeconfig_server
from src.models import ClusterTarget, ConnectResult, EffectiveConfig, NetworkRequirement, OperationError
from src.network import detect_network_requirement
from src.ssh import fetch_remote_file_cached, get_internal_ip, load_ssh_config, make_ssh_client
from src.tunnel import create_tunnel, get_unique_port, save_network_metadata, save_tunnel_pid


def connect_cluster(
    target: ClusterTarget,
    config: EffectiveConfig,
    *,
    allow_manual_network: bool = True,
) -> ConnectResult:
    network_type, network_range, needs_vpn = detect_network_requirement(target.host_config, target.group)
    requirement = NetworkRequirement(type=network_type, network_range=network_range, needs_vpn=needs_vpn)

    if not allow_manual_network and (needs_vpn or network_type == "sshuttle"):
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

    ssh_config = load_ssh_config(target.host_alias, config.ssh_config_path)
    ssh_client = make_ssh_client(
        ssh_config.get("hostname", target.host_alias),
        ssh_config.get("user", "ubuntu"),
        config.ssh_key_path,
        int(ssh_config.get("port", 22)),
        ssh_config.get("proxycommand"),
    )

    try:
        internal_ip = get_internal_ip(ssh_client)
        cache_path = Path.home() / ".cache" / "k9s-config" / f"{target.context_name}.yml"
        content, used_cache = fetch_remote_file_cached(ssh_client, config.remote_k3s_config_path, cache_path)
        local_port = get_unique_port(target.context_name, config.port_range_start, config.port_range_size)
        merged = update_kubeconfig_server(content, internal_ip, config.k3s_api_port, use_localhost=True, local_port=local_port)
        merge_kubeconfig(merged, target.context_name)
        tunnel_pid = create_tunnel(target.host_alias, internal_ip, local_port, config.k3s_api_port)
        save_tunnel_pid(target.context_name, tunnel_pid)
        save_network_metadata(target.context_name, network_type, network_range, None, needs_vpn, internal_ip)
        return ConnectResult(True, target.context_name, local_port, internal_ip, tunnel_pid, used_cache, requirement)
    except Exception as exc:
        return ConnectResult(
            success=False,
            context_name=target.context_name,
            local_port=None,
            internal_ip=None,
            tunnel_pid=None,
            used_cache=False,
            network_requirement=requirement,
            error=OperationError(code="connect_failed", message="Cluster connection failed", detail=str(exc)),
        )
    finally:
        ssh_client.close()
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_services_connect.py tests/unit/test_fetch_k3s_config.py -q`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/services/__init__.py src/services/connect.py src/network.py src/ssh.py src/kubeconfig.py src/tunnel.py tests/unit/test_services_connect.py tests/unit/test_fetch_k3s_config.py
git commit -m "feat: extract cluster connection service"
```

### Task 3: Extract Status, Context, Inventory and Canonical Config Services

**Files:**
- Create: `src/services/status.py`
- Create: `src/services/contexts.py`
- Create: `src/services/inventory_service.py`
- Modify: `src/config.py`
- Modify: `src/inventory.py`
- Modify: `src/network_validator.py`
- Modify: `src/multi_status.py`
- Test: `tests/unit/test_services_status.py`
- Test: `tests/unit/test_services_contexts.py`
- Test: `tests/unit/test_services_inventory_service.py`
- Modify: `tests/unit/test_config.py`
- Modify: `tests/unit/test_inventory.py`

- [ ] **Step 1: Write the failing tests for read-only services and guarded mutation**

```python
from src.models import OperationError
from src.services.contexts import set_current_context
from src.services.inventory_service import list_cluster_targets
from src.services.status import list_context_status


def test_list_cluster_targets_returns_structured_targets(tmp_path):
    inventory_dir = tmp_path / "inventory"
    inventory_dir.mkdir()
    (inventory_dir / "acme_hosts.yml").write_text(
        "all:\n  children:\n    k3s_cluster:\n      hosts:\n        prod:\n          ansible_host: 10.0.0.10\n"
    )

    targets = list_cluster_targets(inventory_dir)
    assert [target.context_name for target in targets] == ["acme-prod"]


def test_set_current_context_requires_explicit_confirmation(mocker):
    mock_run = mocker.patch("src.services.contexts.subprocess.run")

    error = set_current_context("acme-prod", require_confirmation=True, confirmed=False)

    assert isinstance(error, OperationError)
    mock_run.assert_not_called()


def test_list_context_status_reads_tunnel_and_network_metadata(mocker, tmp_path):
    mocker.patch("src.services.status.get_current_context", return_value="acme-prod")
    mocker.patch("src.services.status.list_all_context_names", return_value=["acme-prod"])
    mocker.patch("src.services.status.is_tunnel_running", return_value=True)
    mocker.patch("src.services.status.get_network_metadata", return_value={"network_type": "sshuttle"})

    items = list_context_status(tmp_path)
    assert items[0]["name"] == "acme-prod"
    assert items[0]["is_current"] is True
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_services_status.py tests/unit/test_services_contexts.py tests/unit/test_services_inventory_service.py tests/unit/test_config.py tests/unit/test_inventory.py -q`
Expected: FAIL with missing service modules and missing canonical config helpers

- [ ] **Step 3: Write the minimal implementation**

```python
from __future__ import annotations

import subprocess
from pathlib import Path

from src.config import load_effective_config
from src.inventory import extract_hosts_from_inventory, load_inventories
from src.models import ClusterTarget, EffectiveConfig, OperationError
from src.multi_status import get_current_context
from src.network import detect_network_requirement
from src.network_validator import get_network_metadata, validate_context_network as validate_context_network_rules
from src.tunnel import TUNNEL_STATE_DIR, get_tunnel_pid_file, get_unique_port, is_tunnel_running


def load_effective_config_for_project(project_dir: Path) -> EffectiveConfig:
    raw = load_effective_config(project_dir)
    return raw


def list_cluster_targets(inventory_path: Path) -> list[ClusterTarget]:
    targets: list[ClusterTarget] = []
    for company, inv_data in sorted(load_inventories(inventory_path).items()):
        for host_alias, host_info in sorted(extract_hosts_from_inventory(inv_data).items()):
            targets.append(
                ClusterTarget(
                    company=company,
                    host_alias=host_alias,
                    group=host_info["group"],
                    host_config=host_info["config"],
                )
            )
    return targets


def set_current_context(context_name: str, *, require_confirmation: bool, confirmed: bool) -> OperationError | None:
    if require_confirmation and not confirmed:
        return OperationError(code="confirmation_required", message="Context switch requires explicit confirmation")
    result = subprocess.run(["kubectl", "config", "use-context", context_name], capture_output=True, text=True, timeout=10)
    if result.returncode != 0:
        return OperationError(code="kubectl_context_failed", message="Failed to switch kubectl context")
    return None


def list_context_status(state_dir: Path = TUNNEL_STATE_DIR) -> list[dict[str, object]]:
    current = get_current_context()
    items: list[dict[str, object]] = []
    for pid_file in sorted(state_dir.glob("*.pid")):
        name = pid_file.stem
        items.append(
            {
                "name": name,
                "is_current": name == current,
                "tunnel_running": is_tunnel_running(name, state_dir),
                "local_port": get_unique_port(name),
                "network_metadata": get_network_metadata(name, state_dir),
            }
        )
    return items


def validate_context_network(context_name: str) -> dict[str, object]:
    ok, warning = validate_context_network_rules(context_name)
    return {"ok": ok, "warning": warning}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_services_status.py tests/unit/test_services_contexts.py tests/unit/test_services_inventory_service.py tests/unit/test_config.py tests/unit/test_inventory.py -q`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/services/status.py src/services/contexts.py src/services/inventory_service.py src/config.py src/inventory.py src/network_validator.py src/multi_status.py tests/unit/test_services_status.py tests/unit/test_services_contexts.py tests/unit/test_services_inventory_service.py tests/unit/test_config.py tests/unit/test_inventory.py
git commit -m "feat: add read-only services and guarded context switching"
```

### Task 4: Refactor the Manual CLI to Use the Core

**Files:**
- Modify: `fetch_k3s_config.py`
- Modify: `multi_connect.py`
- Modify: `src/cli.py`
- Test: `tests/unit/test_fetch_k3s_config.py`
- Modify: `tests/unit/test_cli.py`
- Modify: `tests/smoke/test_e2e.py`

- [ ] **Step 1: Write the failing tests for manual flows on top of services**

```python
from src.models import ConnectResult, NetworkRequirement


def test_main_uses_connect_service_instead_of_inline_business_logic(mocker):
    mocker.patch("fetch_k3s_config.select_company", return_value=("acme", {"all": {}}))
    mocker.patch("fetch_k3s_config.select_host", return_value=("prod", {"group": "k3s_cluster", "config": {}}))
    mocker.patch("fetch_k3s_config.connect_cluster", return_value=ConnectResult(
        success=True,
        context_name="acme-prod",
        local_port=16443,
        internal_ip="10.0.0.10",
        tunnel_pid=4242,
        used_cache=False,
        network_requirement=NetworkRequirement.none(),
    ))

    assert fetch_k3s_config.main() == 0


def test_multi_connect_sets_first_successful_context(mocker):
    mocker.patch("multi_connect.list_cluster_targets", return_value=[])
    mocker.patch("multi_connect.connect_multiple", return_value=[])
    mocker.patch("multi_connect.set_current_context")

    assert multi_connect.main() == 0
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_fetch_k3s_config.py tests/unit/test_cli.py tests/smoke/test_e2e.py -q`
Expected: FAIL because the scripts still own business logic directly

- [ ] **Step 3: Write the minimal implementation**

```python
from src.models import ClusterTarget
from src.services.connect import connect_cluster, connect_multiple
from src.services.inventory_service import list_cluster_targets


def build_target(company: str, host_alias: str, host_info: dict) -> ClusterTarget:
    return ClusterTarget(
        company=company,
        host_alias=host_alias,
        group=host_info["group"],
        host_config=host_info["config"],
    )


def main() -> int:
    company, inv_data = select_company(INVENTORY_PATH)
    if company is None:
        return 0

    host_alias, host_info = select_host(company, inv_data)
    if host_alias is None:
        return 0

    target = build_target(company, host_alias, host_info)
    result = connect_cluster(target=target, config=effective_config, allow_manual_network=True)
    if not result.success:
        print(result.error.message)
        return 1

    print(f"Context {result.context_name} ready on localhost:{result.local_port}")
    return 0
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_fetch_k3s_config.py tests/unit/test_cli.py tests/smoke/test_e2e.py -q`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add fetch_k3s_config.py multi_connect.py src/cli.py tests/unit/test_fetch_k3s_config.py tests/unit/test_cli.py tests/smoke/test_e2e.py
git commit -m "refactor: move manual flows onto shared core"
```

### Task 5: Add Structured Logging and Safe Error Translation

**Files:**
- Modify: `src/logging_config.py`
- Modify: `src/services/connect.py`
- Modify: `src/services/status.py`
- Modify: `src/services/contexts.py`
- Modify: `src/services/inventory_service.py`
- Test: `tests/unit/test_logging_config.py`

- [ ] **Step 1: Write the failing tests for structured logging**

```python
import logging

from src.logging_config import get_logger, setup_logging


def test_setup_logging_uses_structured_fields(tmp_path):
    log_file = tmp_path / "service.log"
    logger = setup_logging(log_file=str(log_file), level=logging.INFO, structured=True)
    logger.info("connect_start", extra={"event": "connect_start", "context_name": "acme-prod"})

    content = log_file.read_text()
    assert '"event": "connect_start"' in content
    assert '"context_name": "acme-prod"' in content


def test_safe_error_formatter_does_not_log_secrets(tmp_path):
    log_file = tmp_path / "service.log"
    logger = setup_logging(log_file=str(log_file), level=logging.INFO, structured=True)
    logger.error("ssh_failed", extra={"event": "ssh_failed", "token": "secret-token"})

    content = log_file.read_text()
    assert "secret-token" not in content
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_logging_config.py -q`
Expected: FAIL because logging is plain text only and does not sanitize fields

- [ ] **Step 3: Write the minimal implementation**

```python
import json
import logging


SENSITIVE_KEYS = {"token", "secret", "private_key", "kubeconfig"}


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        payload = {
            "level": record.levelname,
            "message": record.getMessage(),
            "logger": record.name,
        }
        for key, value in record.__dict__.items():
            if key.startswith("_") or key in {"args", "msg", "name", "levelname", "levelno"}:
                continue
            if key in SENSITIVE_KEYS:
                continue
            payload[key] = value
        return json.dumps(payload, sort_keys=True)


def setup_logging(level=logging.DEBUG, verbose=False, log_file=None, structured=False):
    formatter = JsonFormatter() if structured else logging.Formatter("[%(levelname)s] %(message)s")
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_logging_config.py -q`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/logging_config.py src/services/connect.py src/services/status.py src/services/contexts.py src/services/inventory_service.py tests/unit/test_logging_config.py
git commit -m "feat: add structured logging and safe error reporting"
```

### Task 6: Implement the FastMCP Server Layer

**Files:**
- Create: `src/mcp_server.py`
- Modify: `pyproject.toml`
- Test: `tests/unit/test_mcp_server.py`

- [ ] **Step 1: Write the failing tests for tool and resource registration**

```python
from src.mcp_server import build_mcp_server


def test_server_registers_expected_tools():
    server = build_mcp_server()
    tool_names = {tool.name for tool in server._tool_manager.list_tools()}

    assert "connect_cluster" in tool_names
    assert "connect_multiple" in tool_names
    assert "set_current_context" in tool_names
    assert "kill_tunnel" in tool_names
    assert "validate_context_network" in tool_names


def test_server_registers_read_only_resources():
    server = build_mcp_server()
    resource_uris = {resource.uri for resource in server._resource_manager.list_resources()}

    assert "inventory://clusters" in resource_uris
    assert "status://contexts" in resource_uris
    assert "config://effective" in resource_uris
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_mcp_server.py -q`
Expected: FAIL with missing dependency `fastmcp` or missing `src.mcp_server`

- [ ] **Step 3: Write the minimal implementation**

```python
from __future__ import annotations

from pathlib import Path

from fastmcp import Context, FastMCP

from src.config import load_effective_config
from src.models import ClusterTarget
from src.services.connect import connect_cluster, connect_multiple
from src.services.contexts import set_current_context
from src.services.inventory_service import list_cluster_targets
from src.services.status import list_context_status, validate_context_network
from src.tunnel import kill_tunnel


def build_mcp_server() -> FastMCP:
    app = FastMCP("k9s-setup")
    config = load_effective_config(Path.cwd())

    @app.tool(description="Connect one cluster", annotations={"destructiveHint": True, "idempotentHint": False})
    async def connect_cluster_tool(company: str, host_alias: str, ctx: Context) -> dict:
        await ctx.info("connect_cluster started")
        target = next(
            item for item in list_cluster_targets(config.inventory_path)
            if item.company == company and item.host_alias == host_alias
        )
        result = connect_cluster(target=target, config=config, allow_manual_network=False)
        return result.to_public_dict()

    @app.tool(description="Connect multiple clusters", annotations={"destructiveHint": True, "idempotentHint": False})
    async def connect_multiple_tool(targets: list[str], ctx: Context) -> list[dict]:
        await ctx.info("connect_multiple started")
        selected = [
            item for item in list_cluster_targets(config.inventory_path)
            if item.context_name in set(targets)
        ]
        return [item.to_public_dict() for item in connect_multiple(selected, config=config, allow_manual_network=False)]

    @app.tool(description="Switch kubectl context", annotations={"destructiveHint": True, "idempotentHint": True})
    async def set_current_context_tool(context_name: str, confirmed: bool, ctx: Context) -> dict:
        error = set_current_context(context_name, require_confirmation=True, confirmed=confirmed)
        return {"ok": error is None, "error": None if error is None else error.to_public_dict()}

    @app.tool(description="Kill one SSH tunnel", annotations={"destructiveHint": True, "idempotentHint": True})
    async def kill_tunnel_tool(context_name: str, ctx: Context) -> dict:
        kill_tunnel(context_name)
        return {"ok": True, "context_name": context_name}

    @app.tool(description="Validate context network", annotations={"readOnlyHint": True, "openWorldHint": True})
    async def validate_context_network_tool(context_name: str, ctx: Context) -> dict:
        return validate_context_network(context_name)

    @app.resource("inventory://clusters")
    def inventory_clusters() -> list[dict]:
        return [
            {
                "company": target.company,
                "host_alias": target.host_alias,
                "group": target.group,
                "context_name": target.context_name,
            }
            for target in list_cluster_targets(config.inventory_path)
        ]

    @app.resource("status://contexts")
    def status_contexts() -> list[dict]:
        return list_context_status()

    @app.resource("config://effective")
    def effective_config() -> dict:
        return {
            "inventory_path": str(config.inventory_path),
            "ssh_config_path": config.ssh_config_path,
            "ssh_key_path": config.ssh_key_path,
            "remote_k3s_config_path": config.remote_k3s_config_path,
            "k3s_api_port": config.k3s_api_port,
            "port_range_start": config.port_range_start,
            "port_range_size": config.port_range_size,
        }

    return app
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_mcp_server.py -q`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/mcp_server.py pyproject.toml tests/unit/test_mcp_server.py
git commit -m "feat: add FastMCP server layer"
```

### Task 7: Add MCP Entry Points, Documentation and Final Verification

**Files:**
- Modify: `Makefile`
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `tests/smoke/test_e2e.py`

- [ ] **Step 1: Write the failing tests or smoke assertions for the new entry points**

```python
from pathlib import Path


def test_makefile_exposes_mcp_targets():
    makefile = Path("Makefile").read_text()
    assert "mcp-stdio" in makefile
    assert "mcp-http" in makefile


def test_readme_documents_manual_and_mcp_modes():
    readme = Path("README.md").read_text()
    assert "make mcp-stdio" in readme
    assert "make mcp-http" in readme
    assert "set_current_context" in readme
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest tests/smoke/test_e2e.py -q`
Expected: FAIL because the Makefile and docs do not describe MCP mode yet

- [ ] **Step 3: Write the minimal implementation**

```makefile
.PHONY: mcp-stdio mcp-http

mcp-stdio:
	@uv run python -m src.mcp_server stdio

mcp-http:
	@uv run python -m src.mcp_server http --host 127.0.0.1 --port 8000
```

```markdown
## Modo MCP

Executar em `stdio`:

```bash
make mcp-stdio
```

Executar em HTTP:

```bash
make mcp-http
```

Tools mutaveis:
- `connect_cluster`
- `connect_multiple`
- `set_current_context`
- `kill_tunnel`

Tools read-only:
- `validate_context_network`

Resources:
- `inventory://clusters`
- `status://contexts`
- `config://effective`
```

- [ ] **Step 4: Run the full verification suite**

Run: `uv run pytest tests/unit -q`
Expected: PASS

Run: `uv run pytest tests/smoke -q`
Expected: PASS

Run: `uv run mypy src tests`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add Makefile README.md AGENTS.md tests/smoke/test_e2e.py
git commit -m "docs: add MCP entry points and migration guidance"
```

## Notes and Guardrails

- Implement `connect_multiple` inside `src/services/connect.py` as a thin loop over `connect_cluster`; do not duplicate connection logic between manual and MCP layers.
- Keep `questionary` strictly inside `fetch_k3s_config.py`, `multi_connect.py` and `src/cli.py`.
- Keep `k9s-with-tunnel.sh` and `lens-with-tunnel.sh` outside MCP scope.
- Keep `kill_all_tunnels` manual-only; do not expose it in `src/mcp_server.py`.
- Fail MCP connection attempts that require VPN or `sshuttle`; return structured error with explicit remediation instead of prompting.
- For `set_current_context`, require an explicit `confirmed: bool` input in MCP mode.
- Remove the old backup behavior `./<empresa>_<host>.yml` from both manual and MCP paths.
- When touching `pyproject.toml`, add `fastmcp` to runtime dependencies and keep `pytest`, `pytest-mock` and `mypy` in `project.optional-dependencies.dev`.
- Prefer returning `dict` or dataclass-derived structures from tools so clients can consume structured content consistently.
