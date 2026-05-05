# SPEC-005 — Fallback Behavior Contract

## Status
Implementado — `internal/application/usecase/consult_debts.go`

## Motivação

Provedores externos são não-confiáveis: podem ter downtime, lentidão, erros de rede.
O sistema deve ser **resiliente**: o mesmo provedor pode ser **re-tentado** um número limitado de vezes; se continuar falhando, o **próximo** da lista é tentado automaticamente (**fallback**).

## Algoritmo

```
for each provider in ordered_list:
    for attempt = 1 .. maxAttemptsPerProvider:
        if parent_ctx already done:
            return ErrAllProvidersFailed (wrapped)

        ctx_with_timeout = context.WithTimeout(parent_ctx, providerTimeout)
        debts, err = provider.FetchDebts(ctx_with_timeout, plate)
        cancel()

        if err == nil:
            log.Info("provider succeeded", provider, attempt, maxAttempts, latency)
            return debts

        if parent_ctx done:
            return ErrAllProvidersFailed (wrapped)

        if attempt < maxAttemptsPerProvider:
            log.Warn("provider attempt failed, retrying", ...)
            optional fixed backoff (select: sleep or parent_ctx.Done)
            continue

        log.Warn("provider failed, trying next", ...)

return ErrAllProvidersFailed
```

Valores padrão no processo (`cmd/api/main.go`): `providerTimeout` 3s, `maxAttemptsPerProvider` 1 (sem retry), `retryBackoff` 0.

## Timeout por provedor

Cada chamada a `FetchDebts` recebe seu próprio contexto com **timeout por tentativa** (3s por padrão),
independente do contexto pai (requisição HTTP com 10 segundos).

Isso garante que um provider lento não esgota o tempo total disponível
antes do fallback ou do próximo retry ser tentado.

```go
pCtx, cancel := context.WithTimeout(ctx, uc.providerTimeout)
defer cancel()
```

O timeout e a política de retry são injetados via construtor,
permitindo valores diferentes em produção e testes.

## Variáveis de ambiente (processo)

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `PROVIDER_MAX_ATTEMPTS` | `1` | Tentativas por provedor antes de passar ao próximo (mínimo 1). |
| `PROVIDER_RETRY_BACKOFF_MS` | `0` | Pausa fixa entre tentativas no **mesmo** provedor (ms). `0` desliga. |
| `ENABLE_MOCK_FAILING` | `false` | Insere `MockFailing` imediato no início da cadeia (demo de fallback). |
| `ENABLE_MOCK_SLOW` | `false` | Insere `MockFailing{SimulateTimeout: true}` no início (demo de timeout por tentativa). |

Se `ENABLE_MOCK_SLOW` e `ENABLE_MOCK_FAILING` estiverem ambos `true`, **só o modo slow** é registrado e um WARN explica a precedência.

## Comportamentos especificados

| Cenário | Resultado esperado |
|---------|--------------------|
| Provider A disponível | Retorna resultado de A |
| Provider A falha temporariamente e depois ok (com retry) | WARN de retry, depois INFO de sucesso no mesmo A |
| Provider A esgota tentativas, B disponível | WARN em A, INFO em B, retorna resultado de B |
| Todos os providers falham | Retorna `ErrAllProvidersFailed` |
| Provider A excede timeout por tentativa | Erro de contexto, retry ou próximo provedor conforme política |
| Contexto pai cancelado | Não prolonga backoff; retorna `ErrAllProvidersFailed` envolvendo o erro de contexto |

## Logs estruturados por tentativa

Cada tentativa registra campos padronizados (`provider`, `attempt`, `maxAttempts`, `latency`, `plate` mascarada, `error` em falha).

## Demo em tempo real

**Fallback (falha imediata no primeiro provedor):**

```bash
ENABLE_MOCK_FAILING=true make run
# ou
make demo-fallback
```

**Timeout (primeiro provedor segura até o deadline da tentativa):**

```bash
make demo-timeout
# equivalente a ENABLE_MOCK_SLOW=true go run ./cmd/api/...
```

O `MockFailing` é inserido como **primeiro** provider na chain.
Os logs mostram WARN(s) e em seguida INFO do sucesso no próximo provedor saudável.

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
| `TestConsultDebts_Execute_RetryThenSuccess` | Flaky falha 2x, 3ª tentativa ok no mesmo provedor |
| `TestConsultDebts_Execute_RetryExhaustedThenFallback` | Esgota 3 tentativas no flaky → Provider A |
| `TestConsultDebts_Execute_BackoffRespectsCancellation` | Backoff interrompido por `ctx.Cancel` |
| `TestConsultDebts_Execute_PaymentOptions` | Opções de pagamento presentes no resultado |
