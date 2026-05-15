# K3s Context Tunnel Manager

Tool local do workspace `systemframe` para gerenciar túneis SSH e contextos kubeconfig para acesso a clusters K3s via CLI.

## Local certo

```bash
cd /home/helio/Obsidian/02-trabalho/systemframe/custom-tools/k3ctx
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
./k3ctx connect acme prod
```

Regras de uso:

- `clients` mostra apenas clientes e contagem de hosts.
- `hosts <client>` exige escopo de cliente e evita listagem global.
- `connect` aceita 1 a 3 identificadores e conecta apenas quando a resolucao for unica.
- `connect` valida a API Kubernetes forwarded em `https://127.0.0.1:<port>` antes de reportar sucesso.
- `connect` sem identificadores falha com erro deterministico; nao existe prompt interativo.
- se o host tiver `argocd_enabled: true` no inventario, `connect` abre um tunel SSH para o NodePort do ArgoCD e executa `argocd login` automaticamente.
- refresh do inventory nao roda automaticamente; use `--refresh-inventory` ou `K9S_REFRESH_INVENTORY=1` quando quiser atualizar explicitamente.
- `--json` funciona bem sem TTY e retorna saida estruturada (inclui `argocd_local_port` quando disponivel).
- logs da CLI sao legiveis em nivel INFO por padrao; use `K9S_LOG_LEVEL=DEBUG` para debug e `K9S_LOG_FORMAT=json` para logs estruturados no stderr.
- o readiness check da API pode ser ajustado com `K9S_API_READY_TIMEOUT_SECONDS` ou desabilitado com `K9S_VERIFY_API_READY=0`.

`make run` agora chama `connect`. Os atalhos antigos baseados em prompt foram removidos.

## Config canônica

`uv run k3ctx init` prepara o caminho oficial de YAML em:

```bash
~/.local/share/k3ctx/yaml/
```

Estrutura:

```text
~/.local/share/k3ctx/yaml/
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

## Integracao ArgoCD

Adicione ao host no inventario Ansible para ativar login automatico no `connect`:

```yaml
MY-HOST:
  ansible_host: 1.2.3.4
  argocd_enabled: true
  argocd_namespace: argocd      # default "argocd"
  argocd_node_port: 30080       # NodePort do argocd-server (obrigatorio)
  argocd_plaintext: true        # usar --plaintext em vez de --insecure
```

O que acontece no `connect` quando `argocd_enabled: true`:

1. Tunel SSH para K3s API abre em `localhost:<porta-k3s>`
2. Kubeconfig mesclado em `~/.kube/config`
3. Tunel SSH para ArgoCD NodePort abre em `localhost:<porta-argocd>`
4. Senha lida do secret `argocd-initial-admin-secret` via kubectl
5. `argocd login` executado automaticamente
6. Saida mostra `✓ ArgoCD available at localhost:<porta>`

Para matar o tunel ArgoCD separadamente:

```bash
uv run k3ctx tunnel-kill <context>-argocd
```

Se o `argocd` CLI nao estiver instalado ou o secret nao existir, o `connect` ainda completa com sucesso — apenas o login e ignorado e a porta do tunel e reportada para login manual.

## Troubleshooting rapido

- Se `connect` falhar com `Kubernetes API did not become ready on https://127.0.0.1:<port>/version`, teste primeiro com `K9S_API_READY_TIMEOUT_SECONDS=10`.
- Se o tunel estiver funcional mas o readiness check ainda falhar no seu ambiente, use `K9S_VERIFY_API_READY=0` temporariamente e valide com `kubectl --request-timeout=10s get --raw=/version`.
- Se o `argocd login` falhar, o tunel ainda fica aberto. Verifique: `argocd` CLI instalado? Secret `argocd-initial-admin-secret` existe no namespace? `argocd_plaintext` correto para o cluster?
- Para relogar no ArgoCD sem reconectar o cluster: `uv run k3ctx tunnel-kill <context>-argocd` e depois `uv run k3ctx connect <identifier>`.

## Limites e acoes sensiveis

- `connect` abre tunel SSH, le inventario e altera `~/.kube/config`.
- `tunnel-kill` encerra apenas um tunel por contexto; `tunnel-kill-all` encerra todos.
- Se o cluster exigir VPN ou `sshuttle`, a CLI retorna erro estruturado sem prompt interativo.
- Nao versione `~/.local/share/k3ctx/yaml/config/config.yaml`, kubeconfigs gerados, chaves SSH ou estado local.
- Use `k3ctx ...` como alias curto ou `context-tunnel-manager ...`.

## Como funciona no `systemframe`

- O inventario vem de `/home/helio/Work/systemframe/ansible/inventory`
- O contexto final e mesclado em `~/.kube/config`
- Os YAMLs locais ficam em `~/.local/share/k3ctx/yaml/`
- Os PIDs dos tuneis ficam em `~/.local/state/k9s-tunnels`
- Logs locais ficam em `~/.local/state/k9s/`

## Comandos principais

```bash
make init
make sync
make run
make k9s
make status
make tunnel-list
make tunnel-kill CONTEXT=empresa-host
make tunnel-kill-all
make test
```

## Validacao

```bash
uv run python -m pytest tests/unit -q
uv run python -m pytest tests/smoke -q
uv run python -m mypy src tests
```
