# K3s Context Tunnel Manager

Tool local do workspace `systemframe` para gerenciar túneis SSH e contextos kubeconfig para acesso a clusters K3s, em modo manual ou MCP.

## Local certo

```bash
cd /home/helio/Obsidian/work/02-trabalho/systemframe/custom-tools/k3s-context-tunnel-manager
```

## Modos de uso

### Modo manual

Fluxo interativo para operador local:

```bash
make help
make run
make multi-connect
make k9s
make status
```

Requer TTY para `make run` e `make multi-connect`.

### Modo MCP

Servidor FastMCP sobre o core em [`src/mcp_server.py`](/home/helio/Obsidian/work/02-trabalho/systemframe/custom-tools/k3s-context-tunnel-manager/src/mcp_server.py).

```bash
make mcp-stdio
make mcp-http
```

HTTP usa `127.0.0.1:8000` por padrao. Override:

```bash
make mcp-http MCP_HTTP_HOST=0.0.0.0 MCP_HTTP_PORT=9000
```

## Tools e resources expostos

Tools com efeito mutavel:

- `connect_cluster`
- `connect_multiple`
- `set_current_context`
- `kill_tunnel`

Tool de validacao:

- `validate_context_network`

Resources read-only:

- `inventory://clusters`
- `status://contexts`
- `config://effective`

## Config canônica

Copie `.k9s-config-example/config.yaml` para `config.yaml` e ajuste:

```yaml
inventory_path: /caminho/para/inventory
ssh_config_path: ~/.ssh/config
ssh_key_path: ~/.ssh/id_ed25519
remote_k3s_config_path: /etc/rancher/k3s/k3s.yaml
k3s_api_port: 6443
port_range_start: 16443
port_range_size: 10000
```

`config://effective` expõe a configuracao efetiva carregada em runtime.

## Limites e acoes sensiveis

- `connect_cluster` e `connect_multiple` abrem tunel SSH, leem inventario e alteram `~/.kube/config`.
- `set_current_context` troca o contexto atual do `kubectl`; em MCP use confirmacao explicita.
- `kill_tunnel` encerra apenas um tunel por contexto; `tunnel-kill-all` continua manual-only.
- Se o cluster exigir VPN ou `sshuttle`, o modo MCP retorna erro estruturado/remediacao em vez de prompt interativo.
- Nao versione `config.yaml`, kubeconfigs gerados, chaves SSH ou estado local.

## Como funciona no `systemframe`

- O inventario vem de `/home/helio/Work/systemframe/ansible/inventory`
- O contexto final e mesclado em `~/.kube/config`
- Os PIDs dos tuneis ficam em `~/.local/state/k9s-tunnels`
- Logs locais ficam em `~/.local/state/k9s/`

## Comandos principais

```bash
make init
make sync
make run
make multi-connect
make k9s
make status
make tunnel-list
make tunnel-kill CONTEXT=empresa-host
make tunnel-kill-all
make mcp-stdio
make mcp-http
make test
```

## Validacao

```bash
uv run python -m pytest tests/unit -q
uv run python -m pytest tests/smoke -q
uv run mypy src tests
```
