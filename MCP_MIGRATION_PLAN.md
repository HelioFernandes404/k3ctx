# Plano de Migracao para MCP

## Objetivo

Evoluir o projeto para suportar uso manual e uso via MCP, sem quebrar o fluxo atual, isolando o core e usando FastMCP apenas na camada MCP.

## Principios

- Preservar `make run`, `make multi-connect`, `make status` e `make k9s`.
- Nao misturar FastMCP com a logica de SSH, kubeconfig e tunel.
- Separar leitura de mutacao.
- Tratar acoes sensiveis com limites explicitos.
- Toda decisao de MCP deve seguir a documentacao oficial do FastMCP.

## Requisitos para Esta Versao

### Logging

- O servidor MCP deve emitir logs estruturados durante a execucao.
- O desenho deve considerar logs consumiveis por clientes MCP via `log_handler`.
- O uso de logging deve cobrir pelo menos:
  - inicio e fim de operacoes
  - progresso de etapas longas
  - falhas operacionais
  - decisoes relevantes de fluxo
- O servidor deve usar o contexto MCP para logging quando estiver na camada FastMCP.
- O core nao deve depender diretamente de `print`; deve usar logger e retornos estruturados.
- Logs nao devem vazar segredos, kubeconfig bruto, tokens, chaves ou detalhes sensiveis.

### Tools MCP

- Toda tool deve ter assinatura explicita e tipada.
- Nao usar `*args` nem `**kwargs` em tools.
- Tools devem ser pequenas, previsiveis e com input validavel.
- Preferir retorno estruturado com schema claro.
- Quando fizer sentido, usar anotacoes MCP para refletir comportamento real da tool:
  - `readOnlyHint`
  - `destructiveHint`
  - `idempotentHint`
  - `openWorldHint`
- Tools com mutacao devem ser separadas de consultas read-only.
- Tools longas devem reportar progresso.
- Tools devem usar tratamento de erro consistente e seguro.
- Onde houver risco operacional, considerar `ToolError` para expor apenas erros controlados ao cliente.
- Onde fizer sentido, definir timeout por tool.
- Nao expor tools genericas para executar comandos arbitrarios.

## Fase 1: Extrair o Core

Objetivo: tirar a logica de negocio dos scripts.

Entregas:

- Criar uma camada de servicos para:
  - resolucao de config
  - leitura de inventario
  - analise de requirements de rede
  - conexao SSH
  - fetch/cache de kubeconfig
  - merge de kubeconfig
  - criacao/estado de tunel
- Substituir retornos em tupla por estruturas tipadas.
- Remover `print` e `sys.exit` do core.

Resultado esperado:

- `fetch_k3s_config.py` e `multi_connect.py` passam a ser apenas interface/orquestracao.

## Fase 2: Preservar a Interface Manual

Objetivo: manter o uso humano intacto.

Entregas:

- Reapontar `fetch_k3s_config.py` para o novo core.
- Reapontar `multi_connect.py` para o novo core.
- Manter `questionary` so na camada manual.
- Manter `k9s-with-tunnel.sh` fora da camada MCP.

Resultado esperado:

- Mesmo comportamento externo, com codigo interno mais limpo.

## Fase 3: Adicionar Servidor MCP com FastMCP

Objetivo: expor capacidades seguras e previsiveis.

Tools candidatas:

- `connect_cluster`
- `connect_multiple`
- `set_current_context`
- `kill_tunnel`
- `validate_context_network`

Resources candidatas:

- lista de empresas/hosts
- config efetiva
- status dos contextos
- metadata de rede por contexto

Nao expor como tool:

- execucao arbitraria de shell
- abrir `k9s`
- `git pull` implicito sem controle

## Fase 4: Seguranca e Hardening

Objetivo: reduzir risco operacional.

Entregas:

- classificar tools com leitura vs mutacao
- proteger acoes sensiveis
- validar inputs explicitamente
- padronizar erros estruturados
- revisar o tratamento de arquivos sensiveis e logs

## Fase 5: Testes e Integracao Final

Objetivo: deixar revisavel e operavel.

Entregas:

- testes unitarios do core
- testes da camada MCP
- documentacao curta:
  - rodar manualmente
  - rodar como servidor MCP
  - arquitetura em camadas

## Plano de Implementacao por Arquivo

### Arquivos Novos

#### `src/models.py`

Objetivo:

- concentrar tipos e estruturas de retorno do core

Conteudo previsto:

- dataclasses ou modelos tipados para:
  - configuracao efetiva
  - cluster selecionado
  - requirement de rede
  - resultado de conexao
  - status de contexto
  - erros operacionais estruturados

#### `src/services/` ou modulos equivalentes no `src/`

Objetivo:

- separar casos de uso do core da camada de script e da camada MCP

Conteudo previsto:

- `src/services/connect.py`
  - conectar um cluster
  - conectar varios clusters
  - coordenar fetch, merge, tunel e validacoes
- `src/services/status.py`
  - listar contextos
  - obter status e metadata de rede
- `src/services/contexts.py`
  - trocar contexto atual com protecao explicita
- `src/services/inventory_service.py`
  - listar empresas, hosts e clusters prontos para consumo

Observacao:

- se for mais seguro para a primeira iteracao, esses servicos podem nascer como novos modulos `src/connect_service.py`, `src/status_service.py` e similares, sem criar subdiretorio novo.

#### `src/mcp_server.py`

Objetivo:

- expor a interface MCP com FastMCP

Conteudo previsto:

- criacao do `FastMCP`
- configuracao de logging
- tools seguras e tipadas
- resources read-only
- suporte a `stdio` e HTTP
- uso de `Context` para logs e progresso
- uso de anotacoes MCP coerentes com o comportamento real

#### `tests/unit/test_models.py`

Objetivo:

- validar os tipos e contratos estruturados do core

#### `tests/unit/test_services_*.py`

Objetivo:

- validar os casos de uso extraidos do core

#### `tests/unit/test_mcp_server.py`

Objetivo:

- validar registro de tools/resources e comportamento principal da camada MCP

### Arquivos Existentes a Adaptar

#### `fetch_k3s_config.py`

Mudanca prevista:

- deixar de concentrar a logica de negocio
- virar interface manual para:
  - selecionar empresa/host
  - mostrar warnings humanos
  - chamar o core
  - renderizar saida amigavel no terminal

Remocoes previstas:

- nao gravar mais backup `./<empresa>_<host>.yml`

#### `multi_connect.py`

Mudanca prevista:

- virar interface manual de selecao multipla
- delegar conexoes e consolidacao de resultados ao core

#### `src/config.py`

Mudanca prevista:

- formalizar a configuracao canonica na raiz do projeto
- deixar clara a precedence da configuracao
- possivelmente normalizar acesso centralizado para manual e MCP

#### `src/inventory.py`

Mudanca prevista:

- manter parsing e sync do inventario
- expor retornos mais estruturados para consumo pelo core e MCP

#### `src/network.py`

Mudanca prevista:

- manter regras de deteccao
- devolver estruturas ou enums mais claras, em vez de tuplas soltas quando fizer sentido

#### `src/network_validator.py`

Mudanca prevista:

- manter validacao de requirements de rede
- padronizar retorno estruturado para CLI e MCP

#### `src/ssh.py`

Mudanca prevista:

- manter conexao SSH, hash e cache
- reduzir acoplamento com o fluxo de script
- padronizar erros e logs

#### `src/kubeconfig.py`

Mudanca prevista:

- manter transformacao e merge
- revisar logs e retorno estruturado
- garantir que side effects fiquem claros no core

#### `src/tunnel.py`

Mudanca prevista:

- manter lifecycle do tunel
- separar melhor operacoes read-only de operacoes mutaveis
- preparar consumo seguro pelo MCP

#### `src/multi_status.py`

Mudanca prevista:

- deixar de ser a principal fonte da logica de status
- passar a consumir `status service` ou equivalente

#### `src/logging_config.py`

Mudanca prevista:

- ampliar para suportar melhor logging estruturado
- manter integracao com a camada manual
- servir de base para logs do servidor MCP

#### `src/cli.py`

Mudanca prevista:

- permanecer como camada humana/interativa
- nao conter logica de negocio nova

#### `k9s-with-tunnel.sh`

Mudanca prevista:

- manter fora da camada MCP
- possivelmente ajustar apenas se mudar algum path ou contrato de estado local

#### `lens-with-tunnel.sh`

Mudanca prevista:

- manter fora da camada MCP
- sem prioridade na primeira versao, salvo impacto colateral

#### `init.sh`

Mudanca prevista:

- alinhar setup com a configuracao centralizada na raiz do projeto
- remover pressupostos antigos se necessario

#### `Makefile`

Mudanca prevista:

- preservar comandos atuais
- adicionar comandos novos para MCP, por exemplo:
  - `make mcp-stdio`
  - `make mcp-http`

#### `README.md`

Mudanca prevista:

- documentar:
  - modo manual
  - modo MCP
  - config canonica
  - limites e acoes sensiveis

#### `AGENTS.md`

Mudanca prevista:

- atualizar o guia do repositorio para refletir a nova arquitetura em camadas

### Testes Existentes a Revisar

#### `tests/unit/test_fetch_k3s_config.py`

- adaptar para o novo contrato do core e da interface manual

#### `tests/unit/test_cli.py`

- manter foco no comportamento interativo

#### `tests/unit/test_config.py`

- atualizar para a nova regra de configuracao centralizada

#### `tests/unit/test_inventory.py`

- reforcar parsing e sync automatico do inventario

#### `tests/unit/test_network.py`

- manter cobrindo deteccao de requirements

#### `tests/unit/test_ssh.py`

- manter cobrindo conexao, hash e cache

#### `tests/unit/test_kubeconfig.py`

- manter cobrindo reescrita e merge

#### `tests/unit/test_tunnel.py`

- manter cobrindo lifecycle do tunel

#### `tests/unit/test_logging_config.py`

- ampliar para validar requisitos de logging desta versao

#### `tests/smoke/test_e2e.py`

- revisar para refletir:
  - ausencia do backup local `.yml`
  - novo core compartilhado
  - integracao basica do fluxo manual

### Sequencia Recomendada de Edicao

1. `src/models.py`
2. `src/services/...` ou modulos de servico equivalentes
3. adaptacao de `src/config.py`, `src/inventory.py`, `src/network.py`, `src/ssh.py`, `src/kubeconfig.py`, `src/tunnel.py`
4. refatoracao de `fetch_k3s_config.py` e `multi_connect.py`
5. implementacao de `src/mcp_server.py`
6. atualizacao de `Makefile`, `README.md` e `AGENTS.md`
7. ajuste e expansao dos testes

## Ordem Recomendada

1. Extrair core sem mudar comportamento.
2. Adaptar CLI/manual para usar o core.
3. Adicionar MCP read-only primeiro.
4. Adicionar MCP com mutacoes controladas.
5. Endurecer seguranca, erros e testes.

## Riscos Principais

- misturar side effects com a camada MCP
- expor criacao de tunel sem protecao suficiente
- manter inconsistencias entre `config.yaml` local e `~/.k9s-config/config.yaml`
- preservar compatibilidade do fluxo atual enquanto refatora

## Decisoes Confirmadas

Estas decisoes ja vieram das suas respostas e passam a orientar a implementacao.

1. O `git pull` do inventario no modo MCP deve continuar automatico.
2. Criar tunel e alterar `~/.kube/config` no modo MCP nao deve exigir confirmacao extra.
3. A primeira versao do MCP pode expor mutacoes, nao apenas leitura.
4. A configuracao futura deve ficar centralizada em um unico local.
5. O backup local `./<empresa>_<host>.yml` nao deve continuar, nem no manual nem no MCP.
6. `kill_all_tunnels` deve ficar apenas no modo manual.
7. Trocar o contexto atual do `kubectl` no MCP deve exigir confirmacao ou flag de seguranca.
8. O MCP pode conectar varios clusters na primeira versao.
9. Se houver requirement de VPN ou `sshuttle`, a tool MCP deve apenas avisar e falhar.
10. A primeira versao deve suportar ambos os transportes MCP: `stdio` e HTTP.
11. O caminho canonico de configuracao deve ficar na raiz do projeto.

## Pendencias Ainda Abertas

No momento, nao restam pendencias funcionais abertas no plano. Se surgirem novas decisoes de escopo ou seguranca, elas devem ser adicionadas aqui antes da implementacao.

## Boas Praticas de MCP e FastMCP

- Manter tools pequenas, previsiveis e com input explicito.
- Separar claramente resources de leitura e tools com mutacao.
- Nao expor execucao arbitraria de shell como tool generica.
- Marcar e proteger acoes sensiveis com flags, validacao e limites claros.
- Usar retornos estruturados e tipados em vez de texto solto.
- Tratar erros operacionais com mensagens seguras e sem vazar detalhes sensiveis.
- Usar `Context` para logging e progresso em operacoes longas.
- Usar logs estruturados do servidor MCP para facilitar debugging e observabilidade no cliente.
- Preferir um core reutilizavel e manter FastMCP apenas na camada de interface MCP.
- Comecar com capacidades read-only ou de menor risco quando houver incerteza operacional.
- Manter o modo manual como cliente da mesma logica central para evitar duplicacao de comportamento.

## Boas Praticas com Codex como Cliente Principal

Estas praticas foram derivadas da documentacao oficial do Codex e devem influenciar o desenho do servidor MCP.

- O servidor deve funcionar bem tanto em `stdio` quanto em HTTP, porque o Codex suporta os dois modos de configuracao de MCP.
- O setup do MCP deve ser simples de configurar em `~/.codex/config.toml` e, quando fizer sentido, em `.codex/config.toml` do projeto.
- O desenho deve facilitar configuracao explicita de:
  - `enabled_tools`
  - `disabled_tools`
  - `required`
  - `startup_timeout_sec`
  - `tool_timeout_sec`
- O servidor deve assumir que o Codex valoriza configuracao previsivel de:
  - diretorio de trabalho correto
  - permissoes corretas
  - sandbox e approvals coerentes
  - ferramentas e conectores disponiveis
- O servidor deve retornar saidas estruturadas de forma consistente, porque clientes MCP modernos tendem a priorizar `structuredContent` quando ele existe.
- As tools devem ter anotacoes fiéis ao comportamento real, especialmente para leitura vs mutacao e para impacto destrutivo.
- O servidor deve facilitar allowlist de tools, para que o Codex possa expor apenas as capacidades necessarias naquele repo ou perfil.
- O desenho deve evitar depender de configuracao manual frágil fora do projeto, porque a documentacao do Codex destaca que muitos problemas de qualidade sao, na pratica, problemas de setup.

## Fontes

- diagnostico do repositorio local
- indice de documentacao FastMCP: `https://gofastmcp.com/llms.txt`
- documentacao obrigatoria do FastMCP: `https://gofastmcp.com/llms-full.txt`
- documentacao FastMCP sobre server setup, tools, resources, context, transportes e deployment
- documentacao FastMCP sobre server logging e `log_handler`
- documentacao FastMCP sobre tools, schemas, anotacoes, `ToolResult`, `ToolError`, timeouts e contexto
- boas praticas gerais de MCP aplicadas ao desenho: tools pequenas, resources de leitura, retornos estruturados, separacao entre core e interface MCP, e contencao de side effects
- documentacao oficial do Codex sobre configuracao e MCP:
  - `https://developers.openai.com/codex/learn/best-practices/#configure-codex-for-consistency`
  - `https://developers.openai.com/codex/mcp/#connect-codex-to-an-mcp-server`
  - `https://developers.openai.com/codex/config-reference/#configtoml`
  - `https://developers.openai.com/codex/guides/agents-sdk/#running-codex-as-an-mcp-server`
