# Playbook — demonstração alinhada ao HomeTest

Roteiro para apresentar os **requisitos** do [HomeTest.pdf](HomeTest.pdf): consulta 2 provedores (ProviderA JSON + ProviderB XML), normaliza débitos, aplica juros exatos, simula pagamento (PIX 5% + cartão 2.5%/mês), e oferece 3 opções (total + 2 parciais).

**Setup:** Go 1.24+, `REFERENCE_DATE=2024-05-10` (fixa os números), terminal + REST Client.

**Placa:** apenas `ABC1234` (IPVA 1500 venc 2024-01-10 + MULTA 300.50 venc 2024-02-15).

---

## 0. Abertura (1–2 min)

| O que dizer | Onde apontar no repo |
|-------------|----------------------|
| Serviço consulta **múltiplos provedores** (formatos diferentes), **normaliza** para um modelo único, aplica **juros**, simula **PIX e cartão**, oferece **total e parcial por tipo**, com **fallback/retry** e **camadas separadas**. | `README.md`, diagrama de arquitetura |
| O PDF usa exemplo fixo `2024-05-10`; no serviço isso é `REFERENCE_DATE` para reproduzir números. | `cmd/api/main.go` |
| Resposta segue estrutura exata do PDF: `placa`, `debitos[]` (detalhes por débito), `resumo`, `pagamentos.opcoes`. | [SPEC-006](SPEC-006-http-api.md), `docs/Untitled-1.json` |

---

## 1. Happy path — múltiplos provedores e normalização (2–3 min)

**Objetivo:** provar que ProviderA (JSON) e ProviderB (XML) retornam dados equivalentes após normalização.

1. Subir:

   ```bash
   REFERENCE_DATE=2024-05-10 make run
   ```

2. Consultar ABC1234:

   ```bash
   curl -s -X POST http://localhost:3000/api/v1/debts \
     -H "Content-Type: application/json" \
     -d '{"placa":"ABC1234"}' | jq .
   ```

3. **O que mostrar:**
   - **ProviderA**: JSON com campos `vehicle`, `debts[].type`, `debts[].amount`, `debts[].due_date`
   - **ProviderB**: XML com `plate`, `debts>debt>category`, `value`, `expiration`
   - Ambos normalizam → `entity.Debt` (estrutura única)
   - Na prática, ProviderA retorna primeiro; se falhar, fallback para B

4. **Valores esperados:**
   - IPVA: original 1500.00 → atualizado 1800.00 (121 dias, 0.33%/dia com teto 20%)
   - MULTA: original 300.50 → atualizado 555.93 (85 dias, 1%/dia)

---

## 2. Regras de negócio — juros e pagamento (2–3 min)

**Objetivo:** IPVA 0.33%/dia (máx 20%); MULTA 1%/dia (sem máx); PIX 5%; cartão 1x/6x/12x com 2.5%/mês.

1. **JSON retornado:**
   - `debitos[]` — detalha cada débito (valor_original, valor_atualizado, dias_atraso)
   - `resumo` — totais
   - `pagamentos.opcoes[]` — TOTAL, SOMENTE_IPVA, SOMENTE_MULTAS (cada com PIX + cartão)

2. **Cálculos:**
   - **Juros IPVA**: 1500 × 0.33% × 121 dias = 1980, mas limitado a 20% = 1800
   - **Juros MULTA**: 300.50 × 1% × 85 = 255.43 → total 555.93
   - **PIX TOTAL**: 2355.93 × 5% desconto = 2238.13
   - **Cartão 6x**: 2355.93 × (1.025)^6 / 6 = 417.81/parcela

3. **Código:**
   - Juros: `internal/domain/service/interest.go` (Strategy por DebtType)
   - Pagamento: `internal/domain/service/payment.go` (agrupamento + opções)

---

## 3. Isolamento — integração × domínio × pagamento (2 min)

**Objetivo do HomeTest:** isolar integração, domínio e pagamento.

| Camada | Pacotes | Responsabilidade |
|--------|---------|------------------|
| Integração (provedores + HTTP) | `internal/adapters/provider`, `internal/adapters/httpapi` | Wire format, status, parsing |
| Domínio | `internal/domain/entity`, `internal/domain/service` | Tipos canônicos, juros, simulação |
| Orquestração | `internal/application/usecase` | Ordem: buscar → calcular → simular |
| Composição | `cmd/api/main.go` | Só wiring |

Mostrar que **`internal/domain` não importa** HTTP nem adapters (regra do projeto).

---

## 4. Extensibilidade — novo provedor ou nova regra (1–2 min)

**Objetivo do HomeTest:** fácil adicionar provedores e regras.

1. **Novo provedor:** implementar `provider.Provider` (`port.go`), registrar em `buildProviders` em `cmd/api/main.go`.
2. **Nova regra de juros:** novo `DebtType` + nova `InterestStrategy` em `interest.go` + teste em `interest_test.go`.

Oferecer fazer uma alteração **com IA** (ex.: comentar um novo tipo fictício) para cumprir o pedido explícito do PDF sobre uso de IA na apresentação.

---

## 5. Resiliência — fallback (2 min)

**Objetivo do HomeTest:** fallback se um provedor falha; *seria bacana* simular indisponibilidade.

1. Em **outro terminal**:

   ```bash
   make demo-fallback
   ```

2. Repetir o mesmo `curl`.

3. **O que mostrar nos logs (stdout JSON):**
   - WARN no mock / “provider attempt failed” / “trying next”
   - INFO “provider succeeded” no provedor real
   - Placa **mascarada** (`pkg/logger.MaskPlate`)

---

## 6. Timeout e retry (2–3 min)

**Objetivo do HomeTest:** timeout / indisponibilidade; retry ou fallback.

1. **Timeout por tentativa** (mock lento no início da cadeia):

   ```bash
   make demo-timeout
   ```

   Narrar: cada `FetchDebts` tem deadline (3s padrão); o mock segura até estourar → erro de contexto → **fallback** para o próximo provedor.

2. **Retry no mesmo provedor** (opcional, mesmo processo sem mock):

   ```bash
   PROVIDER_MAX_ATTEMPTS=3 PROVIDER_RETRY_BACKOFF_MS=100 REFERENCE_DATE=2024-05-10 make run
   ```

   Narrar: até N tentativas **por provedor** antes de passar ao próximo; backoff opcional. Detalhes: [SPEC-005-fallback.md](SPEC-005-fallback.md).

---

## 7. Validação de entrada (1 min)

**Objetivo:** input validation, erros HTTP, hardening (MaxBytesReader, strict JSON).

Testes na linha de comando:

```bash
# Campo ausente
curl -X POST http://localhost:3000/api/v1/debts \
  -H "Content-Type: application/json" -d '{}' | jq .
# → 400, "campo 'placa' é obrigatório"

# Placa inválida
curl -X POST http://localhost:3000/api/v1/debts \
  -H "Content-Type: application/json" -d '{"placa":"INVALID"}' | jq .
# → 400, "formato de placa inválido"
```

Outros casos (testados em `handler_test.go`):
- Campo desconhecido → 400 (`DisallowUnknownFields`)
- Corpo > 1 MiB → 413 (`MaxBytesReader`)

---

## 8. Testes automatizados (1–2 min)

**Objetivo do HomeTest:** ter testes automatizados.

```bash
make test
make test-race   # se quiser destacar concorrência
make coverage    # opcional
```

**O que dizer:** testes de domínio (`internal/domain/service/*_test.go`), use case (`consult_debts_test.go` — fallback, timeout, retry), handlers (`handler_test.go`), adapters de provedor (`provider_test.go`).

---

## 9. Logs e observabilidade (1 min)

**Objetivo do HomeTest:** demonstrar uso de logs.

1. Subir com `LOG_LEVEL=DEBUG` (se houver mensagens debug relevantes) ou INFO padrão.
2. Mostrar campos estruturados: `provider`, `attempt`, `latency`, `plate` mascarado, `error` em falhas.
3. Origem: `pkg/logger` (JSON para stdout), uso em `consult_debts.go` e `handler.go`.

---

## 10. Padrões de projeto (2 min)

**Objetivo do HomeTest:** explicar padrões (Strategy, Adapter, etc.).

| Padrão / ideia | Onde |
|----------------|------|
| **Strategy** | `InterestStrategy` + `Calculator` por tipo de débito |
| **Adapter** | `ProviderA`, `ProviderB`, `MockFailing` implementam a porta |
| **Port** | `provider.Provider` — domínio/use case dependem da interface |
| **Facade** | `ConsultDebts.Execute` orquestra busca + cálculo + simulação |
| **Fallback / chain** | `fetchWithFallback` — ordem de provedores |
| **Injeção de dependências** | `cmd/api/main.go` constrói tudo explicitamente |

---

## 11. Entrega Git + README (1 min)

**Objetivo do HomeTest:** repositório; README com como rodar, decisões, trade-offs, melhorias.

Apontar seções do `README.md` raiz e docs em `docs/` (SPECs).

---

## Checklist

- [ ] **Happy path:** ABC1234 retorna IPVA 1800.00 + MULTA 555.93 → total 2355.93
- [ ] **Provedores:** ProviderA (JSON) + ProviderB (XML), fallback automático
- [ ] **Juros:** IPVA 0.33%/dia cap 20%, MULTA 1%/dia sem cap
- [ ] **Pagamento:** PIX 5% desconto, cartão 2.5%/mês × 1/6/12
- [ ] **Opções:** TOTAL (2355.93) + SOMENTE_IPVA (1800.00) + SOMENTE_MULTAS (555.93)
- [ ] **Camadas:** adapters / domain / usecase / cmd isolados
- [ ] **Fallback:** `make demo-fallback` mostra WARN + retry
- [ ] **Timeout:** `make demo-timeout` com mock lento
- [ ] **Validação:** campo ausente/inválido → 400, corpo > 1MB → 413
- [ ] **Testes:** `make test`, cobertura
- [ ] **Logs:** estruturados, placa mascarada
- [ ] **Padrões:** Strategy, Adapter, Facade
- [ ] **README:** decisões, trade-offs, melhorias  

---

## Referências no repositório

| Tema | Arquivo |
|------|---------|
| Suite de exemplos HTTP | [`http-requests/api.rest`](../http-requests/api.rest) (happy path + validação + error cases) |
| Fallback + retry + env | [SPEC-005-fallback.md](SPEC-005-fallback.md), `internal/application/usecase/consult_debts.go` |
| HTTP | [SPEC-006-http-api.md](SPEC-006-http-api.md), `internal/adapters/httpapi/handler.go` |
| Porta de provedor | [SPEC-001-provider-port.md](SPEC-001-provider-port.md) |
| Makefile demos | `make demo-fallback`, `make demo-timeout` |
| Logs estruturados | `pkg/logger`, `consult_debts.go`, `handler.go` |
