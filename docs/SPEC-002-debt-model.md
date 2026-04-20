# SPEC-002 — Canonical Debt Model

## Status
Implementado — `internal/domain/entity/debt.go`

## Motivação

Provider A retorna JSON com campos `type`, `amount`, `due_date`.
Provider B retorna XML com campos `category`, `value`, `expiration`.

A camada de domínio não pode conhecer nenhum desses formatos. Precisamos de um
**modelo canônico** único para o qual todos os adapters normalizam seus dados.
Esse é o padrão **Anti-Corruption Layer**: a fronteira entre o mundo externo
(formatos proprietários) e o domínio interno (linguagem ubíqua).

## Tipos definidos

### `DebtType`
```go
type DebtType string

const (
    DebtTypeIPVA  DebtType = "IPVA"
    DebtTypeMULTA DebtType = "MULTA"
)
```
Constantes fortemente tipadas evitam strings mágicas espalhadas pelo código.
Adicionar um novo tipo (ex: `LICENCIAMENTO`) requer apenas uma nova constante aqui.

### `Debt` — modelo canônico de débito
```go
type Debt struct {
    Type    DebtType
    Amount  float64
    DueDate time.Time
}
```
Este é o único tipo que atravessa a fronteira entre adapters e domínio.

### `UpdatedDebt` — débito com juros aplicados
```go
type UpdatedDebt struct {
    Debt
    UpdatedAmount float64
    DaysOverdue   int
}
```
Produzido pelo `InterestCalculator` (SPEC-003). Carrega tanto o valor original
quanto o valor atualizado para compor o resumo da resposta.

### `PaymentSummary`
```go
type PaymentSummary struct {
    TotalOriginal float64 `json:"total_original"`
    TotalUpdated  float64 `json:"total_atualizado"`
}
```

### `Installment`, `PixOption`, `CardOption`
Tipos folha que compõem o `PaymentOption` (ver SPEC-004).

### `PaymentOption`
```go
type PaymentOption struct {
    Type       string     `json:"tipo"`
    BaseAmount float64    `json:"valor_base"`
    Pix        PixOption  `json:"pix"`
    Card       CardOption `json:"cartao_credito"`
}
```
Uma opção de pagamento. O campo `Type` recebe valores como `"TOTAL"`,
`"SOMENTE_IPVA"`, `"SOMENTE_MULTA"` (ver SPEC-007).

### `ConsultResult` — payload de resposta HTTP
```go
type ConsultResult struct {
    Plate   string         `json:"placa"`
    Summary PaymentSummary `json:"resumo"`
    Payment struct {
        Options []PaymentOption `json:"opcoes"`
    } `json:"pagamentos"`
}
```
Este é o tipo retornado pelo use case e serializado pelo handler HTTP.

## Regra de dependência

```
adapters/provider  →  entity.Debt        (normaliza para cá)
domain/service     →  entity.*           (opera sobre estes tipos)
adapters/http      →  entity.ConsultResult (serializa este tipo)
```

Nenhum tipo fora de `entity/` conhece JSON de provedor ou XML de provedor.
