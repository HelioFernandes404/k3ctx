## 1. Domain

- [x] 1.1 Criar `internal/domain/alertmanager.go` com `AlertmanagerConfig` (campos: `Enabled bool`, `Discovery bool`, `Namespace string`, `NodePort *int`) e factories `AutoDiscoverAlertmanagerConfig()`, `DisabledAlertmanagerConfig()`
- [x] 1.2 Adicionar `AlertmanagerLocalPort *int` em `domain.ConnectResultParams` e no resultado JSON de connect

## 2. Application Port

- [x] 2.1 Criar interface `application.AlertmanagerConnector` com método `Setup(contextName string, cfg domain.AlertmanagerConfig, hostname, username string, keyfile *string, port int, proxycmd *string, internalIP string) (AlertmanagerResult, error)`
- [x] 2.2 Criar tipo `application.AlertmanagerResult` com campos `Skipped bool`, `LocalPort *int`, `Message string`

## 3. Infrastructure Adapter

- [x] 3.1 Criar `internal/infrastructure/alertmanager.go` com `LocalAlertmanagerConnector` replicando o padrão de `argocd.go` (campos injetáveis para teste, `StateDir`, `stateDir()` helper)
- [x] 3.2 Implementar `discoverAlertmanagerService(contextName string, kubectlRun func) *discoveredAlertmanagerService` com ranking: label `app.kubernetes.io/name=alertmanager` > nome `alertmanager` > namespace `monitoring` > nome contendo `alertmanager`; porta preferida: port 9093 ou nome `http`
- [x] 3.3 Implementar `Setup()` com: skip se não descoberto, `tunnel.GetUniquePort` na faixa 38000–47999, reuse tunnel se já rodando, abrir novo tunnel se não existe; retornar `AlertmanagerResult` com `LocalPort` e `Message`
- [x] 3.4 Adicionar constantes `alertmanagerPortRangeStart = 38000` e `alertmanagerPortRangeSize = 10000`

## 4. Testes do Adapter

- [x] 4.1 Criar `internal/infrastructure/alertmanager_test.go` cobrindo: serviço com label correto é descoberto, serviço com nome correto é descoberto, namespace `monitoring` é preferido, porta 9093 é preferida, ClusterIP-only é ignorado, falha de kubectl retorna nil
- [x] 4.2 Testar `Setup()`: discovery ausente retorna `Skipped=true`, tunnel existente é reusado, erro de tunnel retorna resultado com mensagem sem falhar

## 5. Bootstrap e Usecase

- [x] 5.1 Em `internal/bootstrap/bootstrap.go`, instanciar `infrastructure.NewLocalAlertmanagerConnector()` e injetar no usecase de connect
- [x] 5.2 Em `internal/application/usecases/connect.go`, invocar `AlertmanagerConnector.Setup()` após ArgoCD (igualmente best-effort); propagar `AlertmanagerLocalPort` para `ConnectResultParams`

## 6. CLI Output

- [x] 6.1 Em `cli/connect.go`, quando `AlertmanagerLocalPort` não é nil, imprimir linha `Alertmanager: http://127.0.0.1:<port>` abaixo da linha de ArgoCD (se houver)
- [x] 6.2 Garantir que a linha não aparece quando `AlertmanagerLocalPort` é nil

## 7. Validação

- [x] 7.1 Rodar `go test ./...` e confirmar zero falhas
- [x] 7.2 Rodar `make build` e confirmar binário gerado sem erros
- [x] 7.3 Conectar em `systemframe-prod-primaria` e confirmar que a linha `Alertmanager: http://127.0.0.1:<port>` aparece no output
- [x] 7.4 Confirmar que `amtool alert query --alertmanager.url http://127.0.0.1:<port>/alertmanager` responde corretamente sem configuração manual adicional
