## 1. tunnel package — CreateKubectlPortForward

- [x] 1.1 Escrever teste `TestCreateKubectlPortForward` em `tunnel_test.go` com kubectl mockado (verificar que chama `kubectl port-forward svc/<name> <local>:<remote> -n <ns> --context <ctx>`)
- [x] 1.2 Implementar `CreateKubectlPortForward(contextName, namespace, serviceName string, localPort, remotePort int) (*int, error)` em `tunnel.go`
- [x] 1.3 Confirmar que `pgrep` encontra o processo kubectl pelo port pattern e salva PID
- [x] 1.4 Rodar `go test ./internal/tunnel/...` verde

## 2. discovery — aceitar ClusterIP como fallback

- [x] 2.1 Estender `discoveredAlertmanagerService` com campos `ServiceName string` e `UseKubectl bool`
- [x] 2.2 Refatorar `discoverAlertmanagerService` para executar 1ª passada (NodePort) e 2ª passada (ClusterIP) usando a mesma função de ranking
- [x] 2.3 Adicionar helper `bestAlertmanagerClusterPort` análogo a `bestAlertmanagerNodePort` mas sem checar `NodePort == 0`
- [x] 2.4 Escrever testes: ClusterIP selecionado quando sem NodePort; NodePort tem precedência sobre ClusterIP; serviço sem porta compatível retorna nil
- [x] 2.5 Rodar `go test ./internal/infrastructure/... -run TestDiscover` verde

## 3. Setup — bifurcar transporte

- [x] 3.1 Atualizar `LocalAlertmanagerConnector.Setup()`: quando `discovered.UseKubectl == true`, chamar `CreateKubectlPortForward` em vez de `tunnel.CreateTunnel`
- [x] 3.2 Garantir que SSH params (`hostname`, `internalIP`, `username`, etc.) são ignorados no path kubectl
- [x] 3.3 Escrever teste de integração: Setup com ClusterIP → chama CreateKubectlPortForward; Setup com NodePort → chama CreateTunnel
- [x] 3.4 Rodar `go test ./internal/infrastructure/... -run TestSetup` verde

## 4. Validação final

- [x] 4.1 Rodar `go test ./...` sem falhas
- [x] 4.2 Rodar `make lint` sem erros novos
- [x] 4.3 Rodar `make build` e `k3ctx connect <cluster>` — confirmar que `alertmanager_local_port` retorna porta válida
- [x] 4.4 Confirmar que `k3ctx status` mostra tunnel `<context>-alertmanager` como `live`
- [x] 4.5 Validar com `amtool --alertmanager.url=http://localhost:<port>/alertmanager cluster` em prod-secundaria e thinkpad-dev-large
