# Documentação de Especificações — car-payment-gateway

Este diretório contém as especificações formais que guiaram a construção do serviço,
seguindo a metodologia **Spec-Driven Development (SDD)**.

## O que é Spec-Driven Development

Antes de escrever qualquer linha de implementação, definimos **contratos**:

1. Tipos de domínio (entidades, enums)
2. Interfaces de porta (ports)
3. Regras de negócio expressas como asserções testáveis
4. Comportamentos de borda documentados como decisões explícitas

Só depois os contratos foram implementados. Os testes validam as specs, não as
implementações — o que significa que trocar um adapter (ex: ProviderA por uma
chamada HTTP real) não quebra nenhum teste de domínio.

## Índice de specs

| Arquivo | Spec | Descrição |
|---------|------|-----------|
| [SPEC-001](./SPEC-001-provider-port.md) | Provider Port | Interface que todo provedor externo deve satisfazer |
| [SPEC-002](./SPEC-002-debt-model.md) | Canonical Debt Model | Modelo canônico de débito normalizado de qualquer provedor |
| [SPEC-003](./SPEC-003-interest-rules.md) | Interest Calculation | Regras de juros por atraso (IPVA com cap, MULTA sem cap) |
| [SPEC-004](./SPEC-004-payment-simulation.md) | Payment Simulation | Desconto PIX e parcelas de cartão de crédito |
| [SPEC-005](./SPEC-005-fallback.md) | Fallback Behavior | Resiliência entre provedores com timeout por contexto |
| [SPEC-006](./SPEC-006-http-api.md) | HTTP API Contract | Contrato de entrada e saída da API REST |
| [SPEC-007](./SPEC-007-partial-payment.md) | Partial Payment Grouping | Opções TOTAL e SOMENTE_<TIPO> geradas automaticamente |
| [SPEC-AMBI](./SPEC-AMBI-ambiguities.md) | Ambiguidades | Discrepâncias encontradas no enunciado e decisões tomadas |

## Ordem de construção (SDD em prática)

```
SPEC-002 (tipos)
    ↓
SPEC-001 (interface de porta)
    ↓
SPEC-003 + SPEC-004 (regras de domínio) → testes escritos primeiro
    ↓
SPEC-007 (agrupamento de pagamento) → testes escritos primeiro
    ↓
SPEC-005 (fallback) → implementação do use case
    ↓
SPEC-006 (contrato HTTP) → adapter HTTP
    ↓
main.go (wiring)
```

## Mapeamento specs → código

| Spec | Arquivo Go |
|------|-----------|
| SPEC-001 | `internal/adapters/provider/port.go` |
| SPEC-002 | `internal/domain/entity/debt.go` |
| SPEC-003 | `internal/domain/service/interest.go` |
| SPEC-004 | `internal/domain/service/payment.go` |
| SPEC-005 | `internal/application/usecase/consult_debts.go` |
| SPEC-006 | `internal/adapters/httpapi/handler.go` |
| SPEC-007 | `internal/domain/service/payment.go` (função `groupByType`) |

---

## Leitura sugerida para revisão (humana ou por ferramenta de IA)

1. **Índice e ordem de construção** — tabelas acima (o que existe e em que ordem foi pensado).
2. **Contratos de API e HTTP** — [SPEC-006](./SPEC-006-http-api.md) + seção **API** no [README raiz](../README.md).
3. **Onde o código diverge do enunciado original** — [SPEC-AMBI](./SPEC-AMBI-ambiguities.md) (obrigatório antes de julgar “bug” em número ou exemplo).
4. **Mapeamento arquivo spec → pacote Go** — coluna da tabela “Mapeamento specs → código”; o diagrama em `README.md` espelha os mesmos pacotes (`httpapi`, não `http`).

Comandos úteis no repositório: `make test`, `make test-race`, `make lint` (`go vet ./...`).
