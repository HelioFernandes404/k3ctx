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
uv run k3ctx init
uv run k3ctx clients
uv run k3ctx hosts acme
uv run k3ctx connect acme prod
uv run k3ctx k9s
uv run k3ctx tunnel-list
uv run k3ctx status

make help
make run
make status
```

Exemplos de refinamento:

```bash
uv run k3ctx hosts acme --host api --limit 10
uv run k3ctx connect --ip 10.0.0.10
uv run k3ctx connect --context acme-prod
uv run k3ctx connect --id sf-1042 --json
uv run k3ctx tunnel-kill acme-prod
uv run k3ctx tunnel-kill-all
uv run k3ctx status --json
```

Aliases compativeis:

```bash
uv run context-tunnel-manager connect acme prod
uv run k3s-context-tunnel-manager connect acme prod
```

Regras de uso:

- `clients` mostra apenas clientes e contagem de hosts.
- `hosts <client>` exige escopo de cliente e evita listagem global.
- `connect` aceita 1 a 3 identificadores e conecta apenas quando a resolucao for unica.
- `connect` valida a API Kubernetes forwarded em `https://127.0.0.1:<port>` antes de reportar sucesso.
- `connect` sem identificadores falha com erro deterministico; nao existe prompt interativo.
- refresh do inventory nao roda automaticamente; use `--refresh-inventory` ou `K9S_REFRESH_INVENTORY=1` quando quiser atualizar explicitamente.
- `--json` funciona bem sem TTY e retorna saida estruturada.
- logs da CLI sao legiveis em nivel INFO por padrao; use `K9S_LOG_LEVEL=DEBUG` para debug e `K9S_LOG_FORMAT=json` para logs estruturados no stderr.
- o readiness check da API pode ser ajustado com `K9S_API_READY_TIMEOUT_SECONDS` ou desabilitado com `K9S_VERIFY_API_READY=0`.

`make run` agora chama `connect`. Os atalhos antigos baseados em prompt foram removidos.

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
uv run k3s-context-tunnel-manager-mcp-stdio
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

`uv run k3ctx init` prepara o caminho oficial de YAML em:

```bash
~/.local/share/k3s-context-tunnel-manager/yaml/
```

Estrutura:

```text
~/.local/share/k3s-context-tunnel-manager/yaml/
├── config/config.yaml
└── kubeconfigs/<context>.yml
```

`XDG_DATA_HOME` altera a base automaticamente. `init` tambem migra:

- `config.yaml` no root do projeto
- `~/.k9s-config/config.yaml`
- kubeconfigs `.yml` ou `.yaml` deixados no root do projeto

Template versionado:

- `examples/config/config.yaml`

Exemplo de `config.yaml`:

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

## Troubleshooting rapido

- Se `connect` falhar com `Kubernetes API did not become ready on https://127.0.0.1:<port>/version`, teste primeiro com `K9S_API_READY_TIMEOUT_SECONDS=10`.
- Se o tunel estiver funcional mas o readiness check ainda falhar no seu ambiente, use `K9S_VERIFY_API_READY=0` temporariamente e valide com `kubectl --request-timeout=10s get --raw=/version`.

## Limites e acoes sensiveis

- `connect_cluster` e `connect_multiple` abrem tunel SSH, leem inventario e alteram `~/.kube/config`.
- `set_current_context` troca o contexto atual do `kubectl`; em MCP use confirmacao explicita.
- `kill_tunnel` encerra apenas um tunel por contexto; `tunnel-kill-all` continua manual-only.
- Se o cluster exigir VPN ou `sshuttle`, CLI/HTTP/MCP retornam erro estruturado/remediacao sem prompt interativo.
- Nao versione `~/.local/share/k3s-context-tunnel-manager/yaml/config/config.yaml`, kubeconfigs gerados, chaves SSH ou estado local.
- Scripts legados fora de `src` foram removidos; use `k3ctx ...` como alias curto ou `context-tunnel-manager ...`, alem de `k3s-context-tunnel-manager-http` e `k3s-context-tunnel-manager-mcp-stdio`.

## Como funciona no `systemframe`

- O inventario vem de `/home/helio/Work/systemframe/ansible/inventory`
- O contexto final e mesclado em `~/.kube/config`
- Os YAMLs locais ficam em `~/.local/share/k3s-context-tunnel-manager/yaml/`
- Os PIDs dos tuneis ficam em `~/.local/state/k9s-tunnels`
- Logs locais ficam em `~/.local/state/k9s/`

## Comandos principais

```bash
make init
make sync
make run
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
uv run python -m mypy src tests
```
