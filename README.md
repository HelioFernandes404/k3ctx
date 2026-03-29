# K9s Setup

Tool local do workspace `systemframe` para buscar kubeconfigs de clusters K3s, criar tunel SSH e abrir o contexto no `kubectl`/`k9s`.

## Local certo

```bash
cd /home/helio/Work/systemframe/.custom-tools/k9s_setup
```

## Fluxo rapido

```bash
make help
make run
make multi-connect
make k9s
make status
```

## Como funciona no `systemframe`

- O inventario vem de `/home/helio/Work/systemframe/ansible/inventory`
- A configuracao local fica em `config.yaml`
- O script atualiza o inventario antes de listar empresas e hosts
- O contexto final e mesclado em `~/.kube/config`
- Os PIDs dos tuneis ficam em `~/.local/state/k9s-tunnels`

## Requisitos

- `uv` instalado
- acesso SSH funcional aos aliases/hosts do inventario
- `~/.ssh/config` e chave SSH configurados
- terminal interativo para `make run` e `make multi-connect`

Se `make run` for executado sem TTY, o comando agora falha com erro claro em vez de stack trace.

## Comandos principais

```bash
make init           # setup inicial
make sync           # sincroniza dependencias com uv
make run            # conecta um cluster
make multi-connect  # conecta varios clusters
make k9s            # abre k9s validando tunel
make status         # mostra clusters conectados
make tunnel-list    # lista tuneis ativos
make tunnel-kill CONTEXT=empresa-host
make tunnel-kill-all
make test
```

## Saidas locais

- `./<empresa>_<host>.yml`: backup standalone do kubeconfig buscado
- `~/.kube/config`: contexto merged para uso com `kubectl`
- `~/.local/state/k9s/k9s-config.log`: log do processo
- `~/.local/state/k9s-tunnels/*.pid`: controle dos tuneis SSH

## Ajustes comuns

Edite `config.yaml` se precisar alterar:

- `inventory_path`
- `ssh_key_path`
- `remote_k3s_config_path`
- `port_range_start`
- `port_range_size`

## Validacao

```bash
uv run --extra dev pytest tests/unit -q
make run
```
