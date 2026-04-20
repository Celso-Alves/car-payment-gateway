# SPEC-005 — Fallback Behavior Contract

## Status
Implementado — `internal/application/usecase/consult_debts.go`

## Motivação

Provedores externos são não-confiáveis: podem ter downtime, lentidão, erros de rede.
O sistema deve ser **resiliente**: se um provedor falha, o próximo é tentado automaticamente.

## Algoritmo

```
for each provider in ordered_list:
    ctx_with_timeout = context.WithTimeout(parent_ctx, 3s)
    debts, err = provider.FetchDebts(ctx_with_timeout, plate)
    cancel()                             // libera recursos imediatamente

    if err == nil:
        log.Info("provider succeeded", provider, latency)
        return debts

    log.Warn("provider failed, trying next", provider, latency, error)

return ErrAllProvidersFailed
```

## Timeout por provedor

Cada tentativa recebe seu próprio contexto com timeout de **3 segundos**,
independente do contexto pai (requisição HTTP com 10 segundos).

Isso garante que um provider lento não esgota o tempo total disponível
antes do fallback ser tentado.

```go
pCtx, cancel := context.WithTimeout(ctx, uc.providerTimeout)
defer cancel()
```

O timeout é injetado via construtor (`providerTimeout time.Duration`),
permitindo valores diferentes em produção e testes.

## Comportamentos especificados

| Cenário | Resultado esperado |
|---------|--------------------|
| Provider A disponível | Retorna resultado de A |
| Provider A falha, B disponível | WARN para A, INFO para B, retorna resultado de B |
| Todos os providers falham | Retorna `ErrAllProvidersFailed` |
| Provider A excede timeout | `context.DeadlineExceeded`, tenta B |
| Contexto pai cancelado | Propaga cancelamento, retorna erro |

## Logs estruturados por tentativa

Cada tentativa registra campos padronizados para facilitar observabilidade:

```json
{"level":"WARN","msg":"provider failed, trying next",
 "provider":"MockFailing","latency":101209625,
 "plate":"ABC1234","error":"context deadline exceeded"}

{"level":"INFO","msg":"provider succeeded",
 "provider":"ProviderA-JSON","latency":170583,
 "plate":"ABC1234"}
```

## Demo em tempo real

Para demonstrar fallback na apresentação:

```bash
ENABLE_MOCK_FAILING=true make run
# ou
make demo-fallback
```

O `MockFailing` é inserido como **primeiro** provider na chain.
Os logs mostram claramente o WARN do fallback seguido do INFO do sucesso.

## `MockFailing` — dois modos

```go
type MockFailing struct {
    SimulateTimeout bool
}
```

| Modo | Comportamento |
|------|--------------|
| `SimulateTimeout: false` | Retorna `ErrProviderUnavailable` imediatamente |
| `SimulateTimeout: true` | Bloqueia até o contexto expirar (`DeadlineExceeded`) |

## Erro sentinela

```go
var ErrAllProvidersFailed = errors.New("all providers failed or are unavailable")
```

O HTTP handler usa `errors.Is(err, ErrAllProvidersFailed)` para retornar
`503 Service Unavailable` quando todos falham.

## Testes relacionados

Arquivo: `internal/application/usecase/consult_debts_test.go`

| Teste | Cobertura |
|-------|-----------|
| `TestConsultDebts_Execute_ProviderASuccess` | Happy path Provider A |
| `TestConsultDebts_Execute_ProviderBSuccess` | Happy path Provider B |
| `TestConsultDebts_Execute_FallbackToProviderB` | MockFailing → fallback para A |
| `TestConsultDebts_Execute_AllProvidersFail` | Todos falham → `ErrAllProvidersFailed` |
| `TestConsultDebts_Execute_ProviderTimeout` | Timeout 100ms → fallback para A |
| `TestConsultDebts_Execute_PaymentOptions` | Opções de pagamento presentes no resultado |
