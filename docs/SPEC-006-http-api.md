# SPEC-006 — HTTP API Contract

## Status

Implementado — `internal/adapters/httpapi/handler.go`, middleware em `internal/adapters/httpapi/middleware.go`.

## Endpoints

### `POST /api/v1/debts`

Consulta débitos e simula opções de pagamento para uma placa.

#### Request

```http
POST /api/v1/debts
Content-Type: application/json

{
  "placa": "ABC1234"
}
```

**Validações:**
- Body JSON válido; campos desconhecidos são rejeitados (`DisallowUnknownFields`) → `400 Bad Request`.
- Tamanho máximo do body: **1 MiB** → `413 Request Entity Too Large` se excedido.
- Campo `placa` não pode ser vazio após `TrimSpace` e `ToUpper` → `400 Bad Request`.
- Placa deve casar com `^[A-Z]{3}-?[0-9][A-Z0-9][0-9]{2}$` (Mercosul ou formato antigo sem hífen) → `400 Bad Request` com mensagem `formato de placa inválido`.
- Se o domínio retornar apenas tipos de débito sem estratégia de juros → `400 Bad Request` (`todos os débitos possuem tipo desconhecido`).

#### Response `200 OK`

Valores monetários serializam como **strings** decimais (tipo `decimal.Decimal`).

```json
{
  "placa": "ABC1234",
  "debitos": [
    {
      "tipo": "IPVA",
      "valor_original": "1500.00",
      "valor_atualizado": "1800.00",
      "vencimento": "2024-01-10",
      "dias_atraso": 121
    },
    {
      "tipo": "MULTA",
      "valor_original": "300.50",
      "valor_atualizado": "543.91",
      "vencimento": "2024-02-19",
      "dias_atraso": 81
    }
  ],
  "resumo": {
    "total_original": "1800.5",
    "total_atualizado": "2343.91"
  },
  "pagamentos": {
    "opcoes": [
      {
        "tipo": "TOTAL",
        "valor_base": "2343.91",
        "pix": {
          "total_com_desconto": "2156.4"
        },
        "cartao_credito": {
          "parcelas": [
            { "quantidade": 1,  "valor_parcela": "2402.51" },
            { "quantidade": 6,  "valor_parcela": "453.04"  },
            { "quantidade": 12, "valor_parcela": "262.69" }
          ]
        }
      },
      {
        "tipo": "SOMENTE_IPVA",
        "valor_base": "1800",
        "pix": {
          "total_com_desconto": "1656"
        },
        "cartao_credito": {
          "parcelas": [
            { "quantidade": 1,  "valor_parcela": "1845" },
            { "quantidade": 6,  "valor_parcela": "347.91" },
            { "quantidade": 12, "valor_parcela": "201.73" }
          ]
        }
      },
      {
        "tipo": "SOMENTE_MULTA",
        "valor_base": "543.91",
        "pix": {
          "total_com_desconto": "500.4"
        },
        "cartao_credito": {
          "parcelas": [
            { "quantidade": 1,  "valor_parcela": "557.51" },
            { "quantidade": 6,  "valor_parcela": "105.13" },
            { "quantidade": 12, "valor_parcela": "60.96" }
          ]
        }
      }
    ]
  }
}
```

*(Exemplo com `REFERENCE_DATE=2024-05-10` no servidor.)*

#### Respostas de erro

| Status | Condição | Body (exemplo) |
|--------|----------|----------------|
| `400` | JSON inválido | `{"error":"invalid JSON body"}` |
| `400` | Campo `placa` vazio | `{"error":"campo 'placa' é obrigatório"}` |
| `400` | Formato de placa inválido | `{"error":"formato de placa inválido"}` |
| `400` | Todos os débitos com tipo desconhecido | `{"error":"todos os débitos possuem tipo desconhecido"}` |
| `413` | Body > 1 MiB | `{"error":"corpo da requisição excede o limite permitido"}` |
| `503` | Todos os provedores falharam | `{"error":"todos os provedores estão indisponíveis"}` |
| `500` | Erro interno inesperado | `{"error":"erro interno"}` |

---

### `GET /health`

Liveness probe. Retorna `200 OK` se o processo está em pé.

```http
GET /health

HTTP/1.1 200 OK
{"status":"ok"}
```

## Comportamentos do handler

- **Timeout:** cada requisição usa `context.WithTimeout` (10s por padrão no `main`, injetado no `NewHandler`).
- **Normalização da placa:** `strings.TrimSpace(strings.ToUpper(plate))` antes de validar e passar ao use case.
- **Content-Type:** todas as respostas têm `Content-Type: application/json`.
- **Logging:** INFO no sucesso, WARN/ERROR na falha, com `plate` **mascarado** (`ABC-****`) e `latency` como campos estruturados.
- **Recover:** panics no mux são capturados, logados com stack, e respondem `500` JSON.

## Configuração

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `PORT` | `3000` | Porta quando `ADDR` não está definida (`:PORT`) |
| `ADDR` | *(vazio)* | Endereço completo do servidor (ex. `:9090`); tem precedência sobre `PORT` |
| `REFERENCE_DATE` | *(vazio)* | Data de referência dos juros (`YYYY-MM-DD`, UTC). Vazio = `time.Now().UTC()` no startup |
| `LOG_LEVEL` | `INFO` | `DEBUG` habilita logs de debug |
| `ENABLE_MOCK_FAILING` | `false` | `true` insere MockFailing (falha imediata) no início da chain |
| `ENABLE_MOCK_SLOW` | `false` | `true` insere MockFailing com simulação de timeout (ver SPEC-005) |
| `PROVIDER_MAX_ATTEMPTS` | `1` | Tentativas por provedor antes do fallback (mínimo 1) |
| `PROVIDER_RETRY_BACKOFF_MS` | `0` | Backoff fixo em ms entre tentativas no mesmo provedor |

## curl de exemplo

```bash
REFERENCE_DATE=2024-05-10 go run ./cmd/api/...

curl -X POST http://localhost:3000/api/v1/debts \
  -H "Content-Type: application/json" \
  -d '{"placa":"ABC1234"}'
```

## Testes relacionados

Arquivo: `internal/adapters/httpapi/handler_test.go`

| Teste | Cobertura |
|-------|-----------|
| `TestHandler_ConsultDebts_Success` | `200` com payload completo |
| `TestHandler_ConsultDebts_MissingPlate` | `400` quando placa está vazia |
| `TestHandler_ConsultDebts_InvalidPlate` | `400` quando placa não casa com o regex |
| `TestHandler_ConsultDebts_InvalidJSON` | `400` quando body não é JSON |
| `TestHandler_ConsultDebts_BodyTooLarge` | `413` quando body > 1 MiB |
| `TestHandler_ConsultDebts_AllProvidersFail` | `503` quando todos falham |
| `TestHandler_Health` | `200` no `/health` |
