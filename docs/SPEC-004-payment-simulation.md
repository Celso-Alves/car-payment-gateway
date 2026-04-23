# SPEC-004 — Payment Simulation Rules

## Status
Implementado — `internal/domain/service/payment.go`

## Entrada

Um `[]entity.UpdatedDebt` produzido pelo `InterestCalculator` (SPEC-003).
Cada item carrega `UpdatedAmount` — o valor com juros já aplicados.

## PIX

```
pix_total = updated_amount × (1 - 0.08)
          = updated_amount × 0.92
```

Desconto fixo de **8%** sobre o valor atualizado (não sobre o valor original).
Aplicado individualmente a cada `PaymentOption` (TOTAL, SOMENTE_IPVA, etc.).

**Exemplo:**
```
base = 2343.91   (TOTAL atualizado)
pix  = 2343.91 × 0.92 = 2156.40
```

## Cartão de crédito

### Fórmula especificada no enunciado

```
installment = valor_total × (1.025)^n / n
```

Onde:
- `valor_total` = valor base da opção de pagamento (já com juros)
- `n` ∈ {1, 6, 12} — número de parcelas
- `1.025` = 1 + taxa mensal de 2,5%

### Parcelas disponíveis

Todas as opções (TOTAL, SOMENTE_IPVA, SOMENTE_MULTA) recebem **1x, 6x e 12x**.
Ver SPEC-AMBI-05 para a decisão sobre SOMENTE_IPVA.

**Exemplo para TOTAL = 2343.91:**

| n | Cálculo | Resultado |
|---|---------|-----------|
| 1 | `2343.91 × 1.025^1 / 1` | 2402.51 |
| 6 | `2343.91 × 1.025^6 / 6` | 453.04  |
| 12 | `2343.91 × 1.025^12 / 12` | 261.99 |

Ver SPEC-AMBI-03 para a discrepância entre esta fórmula e os valores do enunciado.

## Arredondamento

**Implementação:** `github.com/shopspring/decimal` com **`.Round(2)`** (meia unidade afastada
do zero; para montantes positivos, comportamento alinhado ao half-up clássico em BRL).

Aplicado a todos os valores monetários: `UpdatedAmount`, `TotalOriginal`, `TotalUpdated`,
`TotalWithDiscount`, valores de parcela, etc.

Ver SPEC-AMBI-06 para a justificativa e o contrato JSON (strings decimais, sem `float64`).

## Testes relacionados

Arquivo: `internal/domain/service/payment_test.go`

| Teste | Cobertura |
|-------|-----------|
| `TestSimulator_Simulate_PIX` | Desconto PIX 8% correto |
| `TestSimulator_Simulate_CardFormula` | Fórmula `base × 1.025^n / n` verificada |
| `TestSimulator_Simulate_PaymentOptions` | TOTAL + SOMENTE_IPVA + SOMENTE_MULTA presentes |
| `TestSimulator_Simulate_Summary` | TotalOriginal e TotalUpdated corretos |
| `TestSimulator_Simulate_ExtensibleNewType` | LICENCIAMENTO gera SOMENTE_LICENCIAMENTO |
