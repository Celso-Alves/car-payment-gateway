# SPEC-003 — Interest Calculation Rules

## Status
Implementado — `internal/domain/service/interest.go`

## Data de referência fixa

```
2024-05-10
```

Injetada no `Calculator` via construtor, não hard-coded na lógica.
Em produção, substituída por `time.Now()` ou lida de configuração.
Em testes, fixada em `2024-05-10` para resultados determinísticos.

## Fórmula base (juros simples)

```
interest = principal × daily_rate × days_overdue
```

Os exemplos do enunciado confirmam **juros simples**, não compostos.
(Ver SPEC-AMBI-01 para a análise dessa decisão.)

## Regras por tipo de débito

### IPVA

```
daily_rate = 0.0033   (0.33% ao dia)
cap        = principal × 0.20   (20% do valor original)

interest   = min(principal × 0.0033 × days, cap)
updated    = principal + interest
```

**Exemplo do enunciado:**
```
principal = 1500.00
due_date  = 2024-01-10
ref_date  = 2024-05-10
days      = 121

interest_raw = 1500 × 0.0033 × 121 = 599.55
cap          = 1500 × 0.20 = 300.00
interest     = min(599.55, 300.00) = 300.00
updated      = 1500.00 + 300.00 = 1800.00  ✓
```

**Nota real-world:** A legislação SP (SEFAZ-SP) aplica 0,33%/dia por até 60 dias,
depois cap fixo de 20% **mais** a taxa Selic acumulada. O enunciado simplifica
para cap fixo sem Selic — decisão documentada como trade-off no README.

### MULTA

```
daily_rate = 0.01   (1% ao dia)
cap        = sem limite

interest = principal × 0.01 × days
updated  = principal + interest
```

**Exemplo do enunciado:**
```
principal = 300.50
due_date  = 2024-02-19
ref_date  = 2024-05-10
days      = 81  (ver SPEC-AMBI-02)

interest = 300.50 × 0.01 × 81 = 243.405
updated  = 300.50 + 243.41 = 543.91
```

**Nota real-world:** O CTB (art. 131-A) aplica 1% no primeiro mês, depois Selic
acumulada + 1% ao mês. O enunciado simplifica para 1%/dia sem Selic.

## Contagem de dias

```go
func daysOverdue(dueDate, ref time.Time) int {
    due := truncateToDate(dueDate)   // meia-noite UTC
    reference := truncateToDate(ref)
    diff := reference.Sub(due)
    if diff <= 0 {
        return 0
    }
    return int(diff.Hours() / 24)
}
```

Ambas as datas são normalizadas para meia-noite UTC antes da subtração,
garantindo resultado independente de timezone. Com essa normalização, `diff`
é múltiplo de 24h; o número de dias inteiros é `int(diff.Hours() / 24)`.

Ver SPEC-AMBI-02 para a discrepância entre o enunciado (85 dias) e o
resultado matematicamente correto (81 dias) para MULTA.

## Padrão aplicado: Strategy

```
InterestStrategy (interface)
├── ipvaStrategy.Calculate(debt, ref) → (updatedAmount, days)
└── multaStrategy.Calculate(debt, ref) → (updatedAmount, days)
```

`Calculator.Apply()` seleciona a strategy pelo `DebtType` via `map`.
Adicionar `LICENCIAMENTO` = criar `licenciamentoStrategy{}` e registrar no mapa.
Nenhum `if/switch` no Calculator.

## Testes relacionados

Arquivo: `internal/domain/service/interest_test.go`

| Teste | Cobertura |
|-------|-----------|
| `IPVA: 121 days, interest capped at 20%` | Caso exato do enunciado |
| `MULTA: 81 days, no cap` | Cálculo correto com 81 dias |
| `IPVA: interest below cap, not capped` | IPVA sem atingir o cap |
| `IPVA: exactly at 20% cap boundary` | Limiar exato do cap |
| `MULTA: 1 day overdue` | Caso mínimo |
| `Not overdue: due_date = reference` | Sem juros |
| `Not overdue: due_date in future` | Sem juros |
| `MultipleDebts` | IPVA + MULTA em conjunto |
