# K9s Setup

## O que e

Ferramenta local do workspace `systemframe` para:

- buscar kubeconfig de clusters K3s via SSH
- criar tunel local para a API do cluster
- mesclar contexto no `~/.kube/config`
- facilitar o uso com `kubectl` e `k9s`

## Stack

- Python 3
- `uv`
- `paramiko`
- `PyYAML`
- Bash

## Estrutura real

```text
Makefile             # entrada principal
fetch_k3s_config.py  # fluxo de conexao para um cluster
multi_connect.py     # fluxo para varios clusters
k9s-with-tunnel.sh   # abre k9s validando tunel
config.yaml          # configuracao local do projeto
src/                 # modulos de suporte
tests/               # testes unitarios e smoke
```

## Como esse projeto funciona no systemframe

- O inventario vem de `/home/helio/Work/systemframe/ansible/inventory`
- O script tenta atualizar esse inventario antes de listar empresas/hosts
- O backup standalone fica em `./<empresa>_<host>.yml`
- O contexto final vai para `~/.kube/config`
- Os tuneis ficam registrados em `~/.local/state/k9s-tunnels`

## Fluxo recomendado

```bash
cd /home/helio/Work/systemframe/.custom-tools/k9s_setup
make help
make run
make multi-connect
make k9s
make status
```

## Requisitos operacionais

- `uv` instalado
- terminal interativo para `make run` e `make multi-connect`
- acesso SSH funcional
- `~/.ssh/config` ajustado

Se rodar `make run` sem TTY, o comando deve falhar com mensagem clara e sem stack trace.

## Comandos uteis

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
make test
```

## Observacoes

- Nao assumir `inventory/` local: aqui o projeto usa o inventario do `ansible`
- Nao versionar kubeconfigs gerados
- Nao remover a `.venv` por padrao se estiver usando o projeto localmente

## Documentacao

Ver `README.md` para uso rapido e validacao.
