# SPEC-006 — HTTP API Contract

## Status
Implementado — `internal/adapters/http/handler.go`

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
- Body deve ser JSON válido → `400 Bad Request` se inválido.
- Campo `placa` não pode ser vazio após `TrimSpace` e `ToUpper` → `400 Bad Request`.

#### Response `200 OK`

```json
{
  "placa": "ABC1234",
  "resumo": {
    "total_original": 1800.50,
    "total_atualizado": 2343.91
  },
  "pagamentos": {
    "opcoes": [
      {
        "tipo": "TOTAL",
        "valor_base": 2343.91,
        "pix": {
          "total_com_desconto": 2156.40
        },
        "cartao_credito": {
          "parcelas": [
            { "quantidade": 1,  "valor_parcela": 2402.51 },
            { "quantidade": 6,  "valor_parcela": 453.04  },
            { "quantidade": 12, "valor_parcela": 261.99  }
          ]
        }
      },
      {
        "tipo": "SOMENTE_IPVA",
        "valor_base": 1800.00,
        "pix": {
          "total_com_desconto": 1656.00
        },
        "cartao_credito": {
          "parcelas": [
            { "quantidade": 1,  "valor_parcela": 1845.00 },
            { "quantidade": 6,  "valor_parcela": 347.91  },
            { "quantidade": 12, "valor_parcela": 201.73  }
          ]
        }
      },
      {
        "tipo": "SOMENTE_MULTA",
        "valor_base": 543.91,
        "pix": {
          "total_com_desconto": 500.40
        },
        "cartao_credito": {
          "parcelas": [
            { "quantidade": 1,  "valor_parcela": 557.51 },
            { "quantidade": 6,  "valor_parcela": 105.13 },
            { "quantidade": 12, "valor_parcela": 60.96  }
          ]
        }
      }
    ]
  }
}
```

#### Respostas de erro

| Status | Condição | Body |
|--------|----------|------|
| `400` | JSON inválido | `{"error":"invalid JSON body"}` |
| `400` | Campo `placa` vazio | `{"error":"campo 'placa' é obrigatório"}` |
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

- **Timeout:** cada requisição tem `context.WithTimeout(10s)` no handler.
- **Normalização da placa:** `strings.TrimSpace(strings.ToUpper(plate))` antes de passar ao use case.
- **Content-Type:** todas as respostas têm `Content-Type: application/json`.
- **Logging:** INFO no sucesso, WARN/ERROR na falha, com `plate` e `latency` como campos estruturados.

## Configuração

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `PORT` | `8080` | Porta em que o servidor escuta |
| `LOG_LEVEL` | `INFO` | `DEBUG` habilita logs de debug |
| `ENABLE_MOCK_FAILING` | `false` | `true` insere MockFailing no início da chain |

## curl de exemplo

```bash
curl -X POST http://localhost:8080/api/v1/debts \
  -H "Content-Type: application/json" \
  -d '{"placa":"ABC1234"}'
```

## Testes relacionados

Arquivo: `internal/adapters/http/handler_test.go`

| Teste | Cobertura |
|-------|-----------|
| `TestHandler_ConsultDebts_Success` | `200` com payload completo |
| `TestHandler_ConsultDebts_MissingPlate` | `400` quando placa está vazia |
| `TestHandler_ConsultDebts_InvalidJSON` | `400` quando body não é JSON |
| `TestHandler_ConsultDebts_AllProvidersFail` | `503` quando todos falham |
| `TestHandler_Health` | `200` no `/health` |
