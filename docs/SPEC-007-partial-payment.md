# SPEC-007 — Partial Payment Grouping Contract

## Status
Implementado — `internal/domain/service/payment.go` (função `groupByType`)

## Motivação

O sistema deve permitir que o usuário pague **todos os débitos juntos** (TOTAL)
ou **apenas um tipo específico** (ex: só o IPVA, só as multas).

Isso requer agrupar os débitos por `DebtType` e gerar uma opção de pagamento
independente para cada grupo, além da opção consolidada.

## Contrato

Dado um `[]UpdatedDebt`, o `Simulator` deve produzir:

1. Uma opção `TOTAL` com a soma de todos os débitos atualizados.
2. Uma opção `SOMENTE_<DebtType>` para cada tipo de débito distinto presente.

### Rótulos gerados

```
"TOTAL"
"SOMENTE_IPVA"     // quando há ao menos um débito de tipo IPVA
"SOMENTE_MULTA"    // quando há ao menos um débito de tipo MULTA
"SOMENTE_LICENCIAMENTO"  // quando um novo tipo for adicionado — zero mudanças de código
```

### Formato de cada opção

Cada opção de pagamento é gerada pela mesma função `buildOption(label, base)`,
aplicando PIX (SPEC-004) e cartão (SPEC-004) sobre o `base` do grupo.

## Implementação

```go
func groupByType(debts []entity.UpdatedDebt) map[entity.DebtType][]entity.UpdatedDebt {
    m := make(map[entity.DebtType][]entity.UpdatedDebt)
    for _, d := range debts {
        m[d.Type] = append(m[d.Type], d)
    }
    return m
}
```

O `Simulator.Simulate()` itera sobre o map e gera uma opção por entrada,
sem nenhum `if debtType == "IPVA"`. Qualquer novo `DebtType` no sistema
automaticamente produz sua própria opção de pagamento parcial.

## Extensibilidade demonstrada em teste

```go
// TestSimulator_Simulate_ExtensibleNewType
const debtTypeLicenciamento entity.DebtType = "LICENCIAMENTO"
debts := []entity.UpdatedDebt{
    {Debt: entity.Debt{Type: entity.DebtTypeIPVA, ...}, UpdatedAmount: 1100},
    {Debt: entity.Debt{Type: debtTypeLicenciamento, ...}, UpdatedAmount: 180},
}
result := sim.Simulate("XYZ9999", debts)
// → result deve conter uma opção "SOMENTE_LICENCIAMENTO"
```

Este teste passa sem nenhuma mudança no `Simulator` — apenas adicionando um novo
`DebtType` ao sistema o resultado muda.

## Ordem de saída

As opções são ordenadas:
1. `TOTAL` sempre primeiro.
2. Opções parciais em ordem alfabética por tipo (garante saída determinística).

## Relação com SPEC-AMBI-05

O enunciado mostra `SOMENTE_IPVA` com apenas 1 parcela no exemplo de saída,
enquanto `TOTAL` e `SOMENTE_MULTAS` mostram 3 parcelas.

Decisão: ignorar essa assimetria do exemplo (não há regra escrita que a justifique)
e aplicar 1x/6x/12x uniformemente para todas as opções.
Ver [SPEC-AMBI-05](./SPEC-AMBI-ambiguities.md#spec-ambi-05) para a análise completa.

## Testes relacionados

Arquivo: `internal/domain/service/payment_test.go`

| Teste | Cobertura |
|-------|-----------|
| `TestSimulator_Simulate_PaymentOptions` | TOTAL + SOMENTE_IPVA + SOMENTE_MULTA presentes |
| `TestSimulator_Simulate_ExtensibleNewType` | Novo DebtType → nova opção automática |
| `TestSimulator_Simulate_Summary` | Totais original e atualizado corretos |
