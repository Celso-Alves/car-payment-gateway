# SPEC-AMBI — Ambiguidades do Enunciado e Decisões Tomadas

Este documento registra as **discrepâncias encontradas entre as regras escritas
e os exemplos numéricos** do enunciado do home test.

Identificar e documentar essas inconsistências é parte do trabalho de um
engenheiro sênior: a spec raramente está 100% correta, e saber comunicar isso
é tão importante quanto saber codificar.

---

## SPEC-AMBI-01 — Tipo de fórmula de juros (simples vs. composto)

### Problema

O enunciado diz "0,33% ao dia" e "1% ao dia" sem especificar se os juros são
**simples** ou **compostos**.

### Análise

Verificando com o exemplo do enunciado (IPVA, 121 dias):

```
Juros simples:   1500 × 0.0033 × 121 = 599.55  →  cap 20% → 1800.00  ✓
Juros compostos: 1500 × (1.0033^121 - 1) = 870.46  →  cap 20% → 1800.00  (coincide por causa do cap)
```

Para MULTA (81 dias):

```
Juros simples:   300.50 × 0.01 × 81  = 243.41  → total 543.91
Juros compostos: 300.50 × (1.01^81 - 1) = 672.47  → total 972.97  ✗
```

O enunciado mostra ~555.93, confirmando **juros simples**.

### Decisão

Implementar **juros simples**: `interest = principal × rate × days`.
Documentado em SPEC-003.

---

## SPEC-AMBI-02 — Contagem de dias (MULTA: 81 vs. 85 dias)

### Problema

O enunciado afirma "85 dias de atraso" para MULTA com `due_date = 2024-02-19`
e `reference_date = 2024-05-10`. O cálculo matemático correto é **81 dias**.

### Análise

```
Fevereiro (2024 é bissexto, Feb tem 29 dias):
  19/02 → 29/02 = 10 dias restantes

Março:  31 dias
Abril:  30 dias
Maio:   01/05 → 10/05 = 10 dias

Total = 10 + 31 + 30 + 10 = 81 dias
```

`300.50 × 0.01 × 81 = 243.405` → total = **543.91**

O enunciado mostra 555.93 (baseado em 85 dias), mas nenhuma regra de contagem
razoável (inclusiva, exclusiva, por meses completos) produz exatamente 85 dias
para esse par de datas.

### Decisão

Implementar a **subtração de datas matematicamente correta** (81 dias → 543.91).

O total correto do exemplo é `1800.00 + 543.91 = 2343.91`, não 2355.93.

**Comunicação recomendada na apresentação:**
> "Identifiquei que o enunciado usa 85 dias para calcular 555.93, mas a data
> `2024-02-19` a `2024-05-10` resulta em 81 dias. Implementei o cálculo
> matematicamente correto e documentei a discrepância."

---

## SPEC-AMBI-03 — Fórmula do cartão ≠ valores do exemplo

### Problema

O enunciado especifica:
```
installment = valor_total × (1 + 0.025)^n / n
```

Mas os valores no exemplo de saída não correspondem a essa fórmula.

### Análise

Aplicando a fórmula do enunciado ao TOTAL do exemplo (2355.93):

| n | Fórmula literal | Exemplo do enunciado | Bate? |
|---|-----------------|---------------------|-------|
| 1 | 2355.93         | 2355.93             | ✓     |
| 6 | 455.36          | 417.81              | ✗     |
| 12 | 264.03         | 233.07              | ✗     |

A fórmula PMT de amortização Price (`PV × i / (1 - (1+i)^-n)`) também foi testada:
```
n=6:  2355.93 × 0.025 / (1 - 1.025^-6) = 427.70  ≠ 417.81
n=12: 2355.93 × 0.025 / (1 - 1.025^-12) = 229.70 ≠ 233.07
```

Nenhuma fórmula financeira padrão reproduz exatamente os valores do enunciado.
Eles parecem ter sido calculados com uma metodologia não documentada ou com
arredondamentos intermediários.

### Decisão

Implementar a **fórmula literal do enunciado** (`valor_total × 1.025^n / n`),
que é a única especificação escrita disponível. Documentar a discrepância.

**Comunicação recomendada na apresentação:**
> "Percebi que a fórmula escrita `total × (1.025)^n / n` não reproduz os
> valores de exemplo. Implementei a fórmula escrita e questionaria o autor
> sobre como os exemplos foram calculados."

---

## SPEC-AMBI-04 — Typo em SOMENTE_MULTAS 12x

### Problema

O enunciado mostra:
```json
{
  "tipo": "SOMENTE_MULTAS",
  "valor_base": 555.93,
  "cartao_credito": {
    "parcelas": [
      { "quantidade": 12, "valor_parcela": 176.07 }
    ]
  }
}
```

`176.07 × 12 = 2112.84` — que é **3,8× o valor base de 555.93**.

Isso viola matematicamente o conceito de parcelamento: as parcelas nunca devem
totalizar mais do que o valor com juros razoáveis.

### Análise

Usando a fórmula especificada: `555.93 × (1.025)^12 / 12 ≈ 60.96`

O valor correto seria aproximadamente **60.96**, não 176.07.

### Decisão

Ignorar o valor do exemplo (é claramente um erro de digitação/cópia no documento)
e implementar a fórmula especificada. Resultado: ~60.96 por parcela.

---

## SPEC-AMBI-05 — SOMENTE_IPVA com apenas 1 parcela

### Problema

O exemplo de saída mostra `SOMENTE_IPVA` com apenas `quantidade: 1`,
enquanto TOTAL e SOMENTE_MULTAS têm 3 opções (1x, 6x, 12x).

Nenhuma regra escrita no enunciado justifica essa diferença.

### Análise

Hipóteses consideradas:
1. **Regra de domínio não documentada** — IPVA não pode ser parcelado.
2. **Threshold de valor mínimo** — mas IPVA (1800.00) tem valor maior que MULTA (555.93), então não seria threshold.
3. **Erro no exemplo** — mais provável dado o padrão de inconsistências.

**Contexto real-world:** No ecossistema SP, IPVA pode ser parcelado em até 5x
(oficial) ou mais via cartão em instituições credenciadas. Não há razão para
restringir IPVA a 1x no contexto deste teste.

### Decisão

Implementar **uniformemente**: todas as opções de pagamento recebem 1x, 6x e 12x.
Documentar e questionar na apresentação.

**Pergunta para a apresentação:**
> "O exemplo mostra SOMENTE_IPVA com apenas 1 parcela. A regra escrita não
> menciona essa restrição. Implementei 1x/6x/12x para todos os tipos.
> Essa assimetria no exemplo é intencional?"

---

## SPEC-AMBI-06 — Regra de arredondamento não especificada

### Problema

O enunciado usa valores com 2 casas decimais mas não define a regra de arredondamento
(half-up, half-even/banker's, truncamento).

### Decisão

Usar **half-up para 2 casas decimais**: `math.Round(v × 100) / 100`.

É a regra mais intuitiva para valores monetários no contexto brasileiro e
corresponde ao comportamento esperado por usuários finais.

---

## Resumo das decisões

| Ambiguidade | Decisão | Impacto nos números |
|-------------|---------|---------------------|
| AMBI-01 | Juros simples | Confirma valores do enunciado |
| AMBI-02 | 81 dias (correto) | MULTA = 543.91, Total = 2343.91 |
| AMBI-03 | Fórmula literal | Parcelas diferentes do exemplo |
| AMBI-04 | Ignora typo | 12x MULTA ≈ 60.96, não 176.07 |
| AMBI-05 | 1x/6x/12x uniforme | IPVA tem 3 opções, não 1 |
| AMBI-06 | Half-up 2 casas | Consistente com enunciado |
