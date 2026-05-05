# Playbook — demonstração alinhada ao HomeTest

Roteiro para apresentar, em ordem, os **requisitos obrigatórios**, o que seria **bacana** e a **entrega** descritos no [HomeTest.pdf](HomeTest.pdf). Ajuste tempos conforme o slot (sugestão: 15–25 minutos + perguntas).

**Pré-requisitos:** Go 1.24+, terminal, `curl` ou REST Client (VS Code / Cursor extension: `humao.rest-client`), opcionalmente IA (Cursor) para o trecho de “modificação ao vivo”.

**Atalho:** Toda requisição exemplificada neste playbook está em [`http-requests/api.rest`](../http-requests/api.rest). Use “Send All Requests” (botão direito no editor) para rodar a suite sequencial de testes.

---

## 0. Abertura (1–2 min)

| O que dizer | Onde apontar no repo |
|-------------|----------------------|
| Serviço consulta **múltiplos provedores** (formatos diferentes), **normaliza** para um modelo único, aplica **juros**, simula **PIX e cartão**, oferece **total e parcial por tipo**, com **fallback/retry** e **camadas separadas**. | `README.md`, diagrama de arquitetura |
| O PDF usa exemplo fixo `2024-05-10`; no serviço isso é `REFERENCE_DATE` para reproduzir números. | `cmd/api/main.go` |
| Resposta segue estrutura exata do PDF: `placa`, `debitos[]` (detalhes por débito), `resumo`, `pagamentos.opcoes`. | [SPEC-006](SPEC-006-http-api.md), `docs/Untitled-1.json` |

---

## 1. Rodar o happy path — múltiplos provedores e normalização (3–4 min)

**Objetivo do HomeTest:** consultar provedores, normalizar dados, provar que A e B são equivalentes após normalização.

1. Subir sem mocks:

   ```bash
   REFERENCE_DATE=2024-05-10 make run
   ```

2. Consultar (use **exemplo 1** de [`http-requests/api.rest`](../http-requests/api.rest)):

   ```bash
   curl -s -X POST http://localhost:3000/api/v1/debts \
     -H "Content-Type: application/json" \
     -d '{"placa":"ABC1234"}' | jq .
   ```
   
   Ou testar normalização com **exemplo 2** (placa com hífen):
   
   ```bash
   curl -s -X POST http://localhost:3000/api/v1/debts \
     -H "Content-Type: application/json" \
     -d '{"placa":"ABC-1D23"}' | jq .
   ```

3. **O que narrar:**
   - **Provider A** fala JSON (`internal/adapters/provider/provider_a.go`); **Provider B** fala XML (`provider_b.go`).
   - Ambos mapeiam para `entity.Debt` (SPEC-002) — **normalização** no adapter, não no domínio.
   - A cadeia tenta provedores em ordem; com servidor saudável, o **primeiro sucesso** vence (Provider A na prática).
   - Placa com hífen, espaços ou minúsculas (exemplos 2–3 em `api.rest`) são **aceitos e normalizados** antes de consultar provedores.

4. **Opcional (30 s):** abrir um dos arquivos de adapter e mostrar um campo renomeado (`vehicle` vs `plate`, `due_date` vs `expiration`) chegando no mesmo `Debt`.

---

## 2. Regras de negócio — juros e pagamento (2–3 min)

**Objetivo do HomeTest:** IPVA 0,33%/dia com teto 20%; MULTA 1%/dia sem teto; PIX com desconto; cartão 1/6/12x com juros compostos mensais; opções total e parcial.

1. No mesmo `curl`, apontar no JSON:
   - `resumo.total_original` / `total_atualizado`
   - `pagamentos.opcoes` com `TOTAL`, `SOMENTE_IPVA`, `SOMENTE_MULTA` (no código o tipo é `SOMENTE_MULTA`, alinhado à constante `MULTA` — diferença de nomenclatura vs `SOMENTE_MULTAS` do PDF)

2. **Onde está no código:**
   - Juros: `internal/domain/service/interest.go` — **Strategy** por `DebtType`.
   - Pagamento: `internal/domain/service/payment.go` — agrupamento + PIX + parcelas.

3. **Trade-off a mencionar:** o PDF cita desconto PIX de **5%**; a implementação segue SPEC-004 com **8%** (ver comentários no código e README). Se perguntarem: decisão documentada vs enunciado — apontar `SPEC-AMBI` se existir divergência registrada.

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

## 7. Validação de entrada e tratamento de erro (1 min)

**Objetivo do HomeTest:** input validation, mapeamento de erros HTTP, segurança (MaxBytesReader, strict JSON).

Todos os cenários em [`http-requests/api.rest`](../http-requests/api.rest) (exemplos 4–10):
- Placa ausente → 400
- Placa vazia/inválida → 400
- JSON malformado → 400
- Campo desconhecido → 400 (`DisallowUnknownFields`)
- Corpo > 1 MiB → 413 (testado em `handler_test.go:TestHandler_ConsultDebts_BodyTooLarge`)

Ou rapidamente na linha de comando:

```bash
# Campo ausente
curl -X POST http://localhost:3000/api/v1/debts \
  -H "Content-Type: application/json" -d '{}' | jq .

# Placa inválida (ver exemplo 6 em api.rest)
curl -X POST http://localhost:3000/api/v1/debts \
  -H "Content-Type: application/json" -d '{"placa":"INVALID"}' | jq .
```

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

## Checklist rápido (imprimir ou tick na hora)

- [ ] Happy path (`curl` ou `api.rest` exemplo 1–3): múltiplos provedores + normalização  
- [ ] Juros + PIX + cartão + total/parcial (via `pagamentos.opcoes`)  
- [ ] Camadas (adapters / domain / usecase / cmd)  
- [ ] Extensão (porta + registro ou nova strategy)  
- [ ] Fallback (`demo-fallback`) e timeout (`demo-timeout`)  
- [ ] Retry (`PROVIDER_MAX_ATTEMPTS`) mencionado ou demonstrado  
- [ ] Validação (400 / 413): usar `api.rest` exemplos 4–10 ou `curl` quick test  
- [ ] `make test` (e opcionalmente `-race`)  
- [ ] Logs estruturados + placa mascarada (visível em `demo-*`)  
- [ ] Padrões nomeados (Strategy, Adapter, …)  
- [ ] README / decisões / trade-offs  
- [ ] Uso de IA em uma micro-alteração (pedido do PDF)  

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
