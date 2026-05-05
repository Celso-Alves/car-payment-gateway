# Apêndice — exemplos REST e Postman (copiar para os ficheiros oficiais)

O modo *plan* não permite alterar `http-requests/api.rest` nem JSON Postman diretamente.
Copie as secções abaixo para:
- [`http-requests/api.rest`](../http-requests/api.rest)
- [`http-requests/postman/car-payment-gateway.postman_collection.json`](../http-requests/postman/car-payment-gateway.postman_collection.json)

---

## Ficheiro `api.rest` (conteúdo completo sugerido)

```http
# car-payment-gateway — REST Client (VS Code / Cursor extension: humao.rest-client)
#
# Variáveis — ajuste conforme PORT (padrão 3000) ou ADDR no .env
@baseUrl = http://localhost:3000

# ---------------------------------------------------------------------------
# Índice de casos
# ---------------------------------------------------------------------------
# | #   | Caso                                      | HTTP esperado |
# |-----|-------------------------------------------|----------------|
# | 0   | Health liveness                           | 200            |
# | 1   | Consulta Mercosul sem hífen               | 200            |
# | 2   | Mesma lógica com hífen                    | 200            |
# | 3   | Trim + minúsculas                         | 200            |
# | 4–7 | Validação placa / corpo                   | 400            |
# | 8–9 | JSON inválido / não-JSON                  | 400            |
# | 10  | Campo desconhecido (strict JSON)         | 400            |
# | 11  | Rota inexistente                          | 404            |
# | 12  | Método errado em /health                  | 405 ou 404     |
# | 13–15 | Placas exemplo (roadmap DETRAN)       | 200 hoje       |
# | 16  | Com REFERENCE_DATE=2024-05-10 no server | 200 alinhado   |
# | 17–18 | Notas demo fallback / slow            | variável       |
# | 19  | Schema futuro email (400 até implementar)| 400            |
# ---------------------------------------------------------------------------

### 0 — Health (liveness) — esperado: 200, {"status":"ok"}
GET {{baseUrl}}/health

### 1 — Consulta válida (Mercosul, sem hífen) — esperado: 200 + resumo + pagamentos
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "ABC1234"
}

### 2 — Mesma regra com hífen — esperado: 200 (normalização no servidor)
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "ABC-1D23"
}

### 3 — Placa minúscula e espaços — esperado: 200 (TrimSpace + ToUpper)
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "  abc1e34  "
}

### 4 — Validação: campo placa ausente — esperado: 400, "campo 'placa' é obrigatório"
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{}

### 5 — Validação: placa string vazia / só espaços — esperado: 400
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "   "
}

### 6 — Validação: formato de placa inválido — esperado: 400, "formato de placa inválido"
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "INVALID"
}

### 7 — Validação: formato inválido (curto) — esperado: 400
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "AB1234"
}

### 8 — Validação: JSON inválido (sintaxe) — esperado: 400, "invalid JSON body"
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{ placa: "ABC1234" }

### 9 — Validação: corpo não é JSON — esperado: 400
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

not-json-at-all

### 10 — Validação: campo desconhecido (DisallowUnknownFields) — esperado: 400
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "ABC1234",
  "extra": true
}

### 11 — Rota inexistente — esperado: 404
GET {{baseUrl}}/api/v1/inexistente

### 12 — Método não suportado em /health — esperado: 405 ou 404
POST {{baseUrl}}/health
Content-Type: application/json

{}

### 13 — Placa exemplo roadmap DETRAN-SP (2ª placa) — hoje: 200
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "DEF5G67"
}

### 14 — Placa exemplo roadmap DETRAN-RJ — hoje: 200
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "GHI8J90"
}

### 15 — Placa exemplo roadmap DETRAN-RJ (2ª) — hoje: 200
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "JKL1M23"
}

### 16 — Reproduzir números HomeTest — subir servidor com REFERENCE_DATE=2024-05-10
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "ABC1234"
}

### 17 — Com ENABLE_MOCK_FAILING=true no processo: logs WARN, 200 com fallback
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "ABC1234"
}

### 18 — Com ENABLE_MOCK_SLOW=true: ~3s na 1ª tentativa depois 200
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "ABC1234"
}

### 19 — Schema futuro (email): hoje 400 até API aceitar o campo
POST {{baseUrl}}/api/v1/debts
Content-Type: application/json

{
  "placa": "ABC1234",
  "email": "cliente@example.com"
}

# Cenários: REFERENCE_DATE, ENABLE_MOCK_FAILING, ENABLE_MOCK_SLOW, PROVIDER_MAX_ATTEMPTS — ver README / Makefile
# 413 body > 1 MiB: handler_test TestHandler_ConsultDebts_BodyTooLarge
```

---

## Postman — collection JSON completo (`car-payment-gateway.postman_collection.json`)

Substitua o conteúdo do ficheiro Postman por:

```json
{
  "info": {
    "_postman_id": "a1b2c3d4-e5f6-4789-a012-3456789abcde",
    "name": "car-payment-gateway",
    "description": "Chamadas da API car-payment-gateway.\n\nPré-requisito: servidor (`make run`). Variável **baseUrl** (ex.: http://localhost:3000).\n\nCenários no processo (não são headers desta collection):\n- REFERENCE_DATE=2024-05-10 — totais alinhados ao doc de teste\n- ENABLE_MOCK_FAILING=true — demo fallback\n- ENABLE_MOCK_SLOW=true — demo timeout por tentativa\n- PROVIDER_MAX_ATTEMPTS / PROVIDER_RETRY_BACKOFF_MS — retry\n\nItem *Consulta com email* devolve 400 até o schema aceitar o campo (plano agregação/aviso).",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "variable": [{ "key": "baseUrl", "value": "http://localhost:3000" }],
  "item": [
    {
      "name": "Health",
      "item": [
        {
          "name": "Liveness GET /health",
          "request": {
            "method": "GET",
            "header": [],
            "url": "{{baseUrl}}/health",
            "description": "Esperado: 200, {\"status\":\"ok\"}"
          }
        }
      ]
    },
    {
      "name": "Consultas — sucesso (200)",
      "item": [
        {
          "name": "01 Placa ABC1234",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"ABC1234\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "02 Placa com hífen ABC-1D23",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"ABC-1D23\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "03 Minúsculas e espaços",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"  abc1e34  \"\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "13 Placa exemplo DEF5G67 (roadmap SP)",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"DEF5G67\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "14 Placa exemplo GHI8J90 (roadmap RJ)",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"GHI8J90\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "15 Placa exemplo JKL1M23 (roadmap RJ)",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"JKL1M23\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "16 ABC1234 (use REFERENCE_DATE no servidor)",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"ABC1234\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts",
            "description": "Subir API com REFERENCE_DATE=2024-05-10 para alinhar totais ao HomeTest."
          }
        },
        {
          "name": "17 Demo fallback (env no servidor)",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"ABC1234\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts",
            "description": "Processo com ENABLE_MOCK_FAILING=true — logs WARN + 200."
          }
        },
        {
          "name": "18 Demo timeout (env no servidor)",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"ABC1234\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts",
            "description": "Processo com ENABLE_MOCK_SLOW=true — primeira tentativa ~3s."
          }
        }
      ]
    },
    {
      "name": "Consultas — validação (400)",
      "item": [
        {
          "name": "04 Placa ausente {}",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "05 Placa vazia",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"   \"\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "06 Formato inválido INVALID",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"INVALID\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "07 Placa curta AB1234",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"AB1234\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "08 JSON sintaxe inválida",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{ placa: \"ABC1234\" }" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "09 Corpo não JSON",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "not-json-at-all" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        },
        {
          "name": "10 Campo extra (strict)",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"ABC1234\",\n  \"extra\": true\n}" },
            "url": "{{baseUrl}}/api/v1/debts"
          }
        }
      ]
    },
    {
      "name": "Rotas e método",
      "item": [
        {
          "name": "11 GET rota inexistente 404",
          "request": { "method": "GET", "header": [], "url": "{{baseUrl}}/api/v1/inexistente" }
        },
        {
          "name": "12 POST /health método errado",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{}" },
            "url": "{{baseUrl}}/health",
            "description": "Esperado: 405 ou 404 conforme Go mux."
          }
        }
      ]
    },
    {
      "name": "Roadmap — schema futuro",
      "item": [
        {
          "name": "19 Consulta com email (400 até API aceitar)",
          "request": {
            "method": "POST",
            "header": [{ "key": "Content-Type", "value": "application/json" }],
            "body": { "mode": "raw", "raw": "{\n  \"placa\": \"ABC1234\",\n  \"email\": \"cliente@example.com\"\n}" },
            "url": "{{baseUrl}}/api/v1/debts",
            "description": "Hoje: 400 (DisallowUnknownFields). Após plano agregação/aviso Detran: 200 com aviso quando aplicável."
          }
        }
      ]
    }
  ]
}
```

---

Quando puder usar **Agent mode**, peça para aplicar este apêndice diretamente nos ficheiros `http-requests/api.rest` e `car-payment-gateway.postman_collection.json`.
