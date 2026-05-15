## Why

Connecting to a cluster hoje requer 4 passos manuais para acessar alertas: connect, descobrir o serviço, abrir port-forward, configurar amtool. O ArgoCD já resolve esse fluxo automaticamente — o Alertmanager deve seguir o mesmo padrão, eliminando custo de interação e de tokens Claude.

## What Changes

- `k3ctx connect` detecta automaticamente um serviço Alertmanager no cluster após estabelecer acesso Kubernetes
- Se encontrado, abre um managed SSH tunnel para o NodePort do Alertmanager
- Imprime a URL local (`http://127.0.0.1:<port>`) ao usuário junto com o resultado de connect
- A URL local fica disponível no JSON de saída do connect para automações
- O campo `alertmanager_local_port` é adicionado ao resultado JSON de connect
- A discovery é best-effort: falha não bloqueia o connect

## Capabilities

### New Capabilities

- `alertmanager-auto-discovery`: Descoberta automática do serviço Alertmanager e abertura de managed tunnel durante `connect`, espelhando o padrão ArgoCD

### Modified Capabilities

- `cluster-connection`: O resultado do connect ganha o campo `alertmanager_local_port` no JSON e a mensagem de sucesso reporta a URL do Alertmanager quando disponível

## Impact

- `internal/infrastructure/alertmanager.go` — novo adapter (espelha `argocd.go`)
- `internal/domain/` — novo tipo `AlertmanagerConfig` e campos no resultado de connect
- `internal/application/` — porta `AlertmanagerConnector` e resultado `AlertmanagerResult`
- `internal/bootstrap/bootstrap.go` — wire do novo connector
- `internal/application/usecases/connect.go` — invocação da discovery após Kubernetes context pronto
- `cli/connect.go` — exibe URL do Alertmanager no output
- Port range: usa faixa dedicada (ex: 29000–29999) separada da faixa ArgoCD (28000–28999)
