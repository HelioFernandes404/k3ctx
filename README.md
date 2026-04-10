# K3s Context Tunnel Manager

Tool local do workspace `systemframe` para gerenciar túneis SSH e contextos kubeconfig para acesso a clusters K3s, com CLI oficial, interface HTTP e interface MCP reaproveitando o mesmo dominio/aplicacao.

## Local certo

```bash
cd /home/helio/Obsidian/work/02-trabalho/systemframe/custom-tools/k3s-context-tunnel-manager
```

## Modos de uso

### CLI oficial

Fluxo principal para descoberta e conexao:

```bash
uv run context-tunnel-manager clients
uv run context-tunnel-manager hosts acme
uv run context-tunnel-manager connect acme prod
uv run context-tunnel-manager status

make help
make run
make multi-connect
make status
```

Exemplos de refinamento:

```bash
uv run context-tunnel-manager hosts acme --host api --limit 10
uv run context-tunnel-manager connect --ip 10.0.0.10
uv run context-tunnel-manager connect --id sf-1042 --json
```

Aliases legados ainda disponiveis temporariamente:

```bash
uv run context-tunnel-manager single
uv run context-tunnel-manager multi
```

`make run` agora chama `connect`. `make multi-connect` continua disponivel como fluxo legado. Comandos com `--json` funcionam bem sem TTY.

### HTTP

Interface REST sobre os mesmos use cases:

```bash
uv run k3s-context-tunnel-manager-http --host 127.0.0.1 --port 8080
make http
```

Endpoints expostos:

- `GET /config`
- `GET /clusters`
- `GET /status`
- `POST /connect`
- `POST /contexts/current`
- `POST /tunnels/{context}/kill`

Exemplos:

```bash
curl http://127.0.0.1:8080/config
curl http://127.0.0.1:8080/clusters
curl http://127.0.0.1:8080/status
curl -X POST http://127.0.0.1:8080/connect -H 'content-type: application/json' -d '{"context_name":"acme-prod"}'
curl -X POST http://127.0.0.1:8080/contexts/current -H 'content-type: application/json' -d '{"context_name":"acme-prod","require_confirmation":false,"confirmed":true}'
curl -X POST http://127.0.0.1:8080/tunnels/acme-prod/kill
```

### Modo MCP

Servidor FastMCP sobre a mesma base de casos de uso, exposto em [`src/mcp_server.py`](/home/helio/Obsidian/work/02-trabalho/systemframe/custom-tools/k3s-context-tunnel-manager/src/mcp_server.py).

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

## Contratos HTTP alinhados ao MCP

- `GET /config` retorna o mesmo payload de `config://effective`
- `GET /clusters` retorna o mesmo payload de `inventory://clusters`
- `GET /status` retorna o mesmo payload de `status://contexts`
- `POST /connect` retorna o mesmo payload estrutural de `connect_cluster`
- `POST /contexts/current` retorna o mesmo payload estrutural de `set_current_context`
- `POST /tunnels/{context}/kill` retorna o mesmo payload estrutural de `kill_tunnel`

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
make http
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
