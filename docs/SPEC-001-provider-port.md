# SPEC-001 — Provider Port Contract

## Status
Implementado — `internal/adapters/provider/port.go`

## Motivação

O sistema deve consultar múltiplos provedores externos de débitos veiculares.
Cada provedor retorna dados em formatos diferentes (JSON, XML, futuramente SOAP, gRPC, etc.).

Para que o use case nunca dependa de um provedor específico, definimos uma **interface de porta**
que todos os provedores devem satisfazer. Isso é o padrão **Ports & Adapters** (Arquitetura Hexagonal).

## Contrato

```go
type Provider interface {
    Name() string
    FetchDebts(ctx context.Context, plate string) ([]entity.Debt, error)
}
```

### `Name() string`
- Retorna um identificador legível por humanos.
- Usado em logs estruturados: `slog.String("provider", p.Name())`.
- Não afeta lógica de negócio.

### `FetchDebts(ctx context.Context, plate string) ([]entity.Debt, error)`
- Recebe um contexto com timeout (ver SPEC-005).
- Retorna `[]entity.Debt` normalizado no modelo canônico (ver SPEC-002).
- Retorna erro em qualquer falha: timeout, parse error, indisponibilidade.
- O chamador (use case) trata o erro como sinal para tentar o próximo provedor.

## Regras de conformidade

Todo provedor **deve**:
1. Respeitar o cancelamento do contexto (`ctx.Err() != nil` → retornar erro imediatamente).
2. Retornar o modelo canônico `entity.Debt` — nenhum tipo proprietário pode vazar para fora do adapter.
3. Envolver erros com o nome do provedor para facilitar diagnóstico: `fmt.Errorf("%s: %w", p.Name(), err)`.

## Implementações registradas

| Struct | Arquivo | Formato |
|--------|---------|---------|
| `ProviderA` | `provider_a.go` | JSON — simula API SEFAZ-SP |
| `ProviderB` | `provider_b.go` | XML — simula integração legada DETRAN-SP |
| `MockFailing` | `mock_failing.go` | Sempre falha — demo de fallback |

## Extensibilidade

Adicionar um **Provider C** requer apenas:
1. Criar `provider_c.go` que implemente `Provider`.
2. Adicioná-lo ao slice em `cmd/api/main.go`.

Nenhum outro arquivo precisa ser alterado.

## Testes relacionados

- `internal/adapters/provider/provider_test.go`
  - `TestProviderA_FetchDebts`
  - `TestProviderB_FetchDebts`
  - `TestProviderA_CancelledContext`
  - `TestProviderB_CancelledContext`
  - `TestMockFailing_AlwaysFails`
  - `TestMockFailing_SimulateTimeout`
