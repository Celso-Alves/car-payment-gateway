# car-payment-gateway

A vehicle debt consultation and payment simulation service, built as a backend engineering home test.

## How to run

### Prerequisites
- Go 1.22+
- Docker (optional)

### Local

```bash
# Clone and enter the repo
git clone https://github.com/celsoadsjr/car-payment-gateway
cd car-payment-gateway

# Run
make run          # starts on :8080

# Test
make test         # all tests, no verbose
make test-verbose # with output

# Build binary
make build        # outputs to bin/
```

### Docker

```bash
make docker-run   # builds image and runs on :8080
```

### Demo: fallback in action

```bash
make demo-fallback
# Starts with MockFailing provider first in the chain.
# Logs will show a WARN for MockFailing, then INFO for ProviderA succeeding.
```

## API

### `POST /api/v1/debts`

Consults debts and simulates payment options for a vehicle plate.

**Request:**
```json
{ "placa": "ABC1234" }
```

**Response (200):**
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
        "pix": { "total_com_desconto": 2156.40 },
        "cartao_credito": {
          "parcelas": [
            { "quantidade": 1,  "valor_parcela": 2403.51 },
            { "quantidade": 6,  "valor_parcela": 453.22  },
            { "quantidade": 12, "valor_parcela": 262.54  }
          ]
        }
      },
      {
        "tipo": "SOMENTE_IPVA",
        "valor_base": 1800.00,
        ...
      },
      {
        "tipo": "SOMENTE_MULTA",
        "valor_base": 543.91,
        ...
      }
    ]
  }
}
```

**Error responses:**

| Status | Reason |
|--------|--------|
| 400 | Missing or invalid `placa` field |
| 503 | All providers unavailable |
| 500 | Unexpected internal error |

### `GET /health`

Returns `{"status":"ok"}` for liveness probes.

---

## Architecture

The project follows **Clean Architecture** (also called Hexagonal / Ports & Adapters):

```
cmd/api/              ← entry point, wiring only
internal/
  domain/             ← pure business logic, no imports from outer layers
    entity/           ← canonical data models (Debt, PaymentOption, …)
    service/          ← InterestCalculator, Simulator (domain services)
  application/
    usecase/          ← ConsultDebts: orchestrates providers + domain
  adapters/
    provider/         ← ProviderA (JSON), ProviderB (XML), MockFailing
    http/             ← HTTP handler, request/response translation
pkg/
  logger/             ← structured slog wrapper
```

**Dependency rule:** inner layers never import outer layers.
`domain` knows nothing about HTTP or providers.
`application` knows only ports (interfaces), never concrete adapters.

---

## Spec-Driven Development

I applied **Spec-Driven Development**: before writing any implementation,
I defined all contracts as Go interfaces and types:

| Spec | Description |
|------|-------------|
| SPEC-001 | `provider.Provider` interface (port all providers must satisfy) |
| SPEC-002 | `entity.Debt` canonical model (normalised from any provider format) |
| SPEC-003 | Interest rules: simple interest, IPVA 0.33%/day cap 20%, MULTA 1%/day |
| SPEC-004 | Payment rules: PIX 8% discount, card `base*(1.025)^n/n` for n∈{1,6,12} |
| SPEC-005 | Fallback contract: try providers in order, first success wins |
| SPEC-006 | HTTP contract: `POST /api/v1/debts`, JSON in/out |
| SPEC-007 | Partial payment: TOTAL + one option per DebtType, auto-generated |

Tests are written against specs, not implementations — they pass regardless
of which concrete provider or strategy is used.

---

## Design Patterns

| Pattern | Where | Why |
|---------|-------|-----|
| **Adapter** | `ProviderA`, `ProviderB` | Each normalises a different wire format (JSON/XML) into `entity.Debt`. Adding a Provider C = one new file. |
| **Strategy** | `InterestCalculator` | Each `DebtType` has its own strategy (`ipvaStrategy`, `multaStrategy`). New type = new strategy, zero changes elsewhere. |
| **Facade** | `ConsultDebtsUseCase` | Single entry point hiding provider orchestration, fallback, interest calculation, and payment simulation. |
| **Factory (by injection)** | `main.go` | Provider slice assembled at startup. Adding a provider = one line in `main.go`. |
| **Group-by** | `Simulator` | Debts grouped by `DebtType` via `map`, producing one partial option per type automatically — no `if debtType == "IPVA"` conditionals. |

---

## Trade-offs & Decisions

### Interest model simplified vs. real SP legislation

The test uses a simplified model:

| Type | Test rule | Real SP rule |
|------|-----------|--------------|
| IPVA | 0.33%/day, hard 20% cap | 0.33%/day up to 60 days → fixed 20% + **Selic rate** |
| MULTA | 1%/day, no cap | 1st month flat 1% → Selic accumulated + 1%/month (CTB art. 131-A) |

Production would use the monthly Selic rate table published by SEFAZ-SP.

### Day counting (SPEC-AMBI-02)

The test document states "85 days" for 2024-02-19 → 2024-05-10.
Correct UTC date-diff gives **81 days** (Feb has 29 days in 2024).
`81 days × 0.01 × R$300.50 = R$543.91` (spec shows R$555.93 based on 85 days).

Decision: implement mathematically correct date subtraction (81 days).
The spec's day count appears to be a manual counting error. Unit tests pin the behaviour and the discrepancy is noted explicitly.

### Card installment formula (SPEC-AMBI-03)

The spec states: `installment = valor_total * (1.025)^n / n`

Applying this to the spec's own example (TOTAL = R$2343.91):

| n | Formula result | Spec example |
|---|---------------|--------------|
| 1 | 2343.91 | 2355.93\* |
| 6 | 452.24 | 417.81 |
| 12 | 261.99 | 233.07 |

\*Spec total also differs because it uses 85-day MULTA (see above).

The examples don't match the stated formula. The Price/PMT amortisation
formula (`PV * i / (1 - (1+i)^-n)`) also doesn't produce the spec's numbers.
Decision: implement the **literal written formula** and document the gap.

### SOMENTE_IPVA installment count (SPEC-AMBI-05)

Spec expected output shows `SOMENTE_IPVA` with only 1 installment, while
TOTAL and SOMENTE_MULTA show 3. No written rule restricts installments by
debt type. Decision: offer 1x/6x/12x consistently for all options.

### Rounding (SPEC-AMBI-06)

Half-up rounding to 2 decimal places via `math.Round(v*100)/100`.
Consistent across all monetary values.

### Input: `placa` only

Real systems use RENAVAM (11-digit) + plate for secure identification.
Future improvement: accept RENAVAM as an optional field.

### In-memory providers

Providers hold hard-coded payloads matching the spec. In production,
`FetchDebts` would make an authenticated HTTP call to:
- Provider A → SEFAZ-SP API (`integrador.sp.gov.br`)
- Provider B → DETRAN-SP legacy XML endpoint

### No database, no auth

As specified. The service is stateless; results are computed on every request.

---

## Future improvements

- **Real provider HTTP clients** — replace in-memory payloads with actual
  HTTP calls to SEFAZ-SP and DETRAN-SP APIs (bilateral agreement required).
- **RENAVAM input** — accept RENAVAM alongside plate for precise lookup.
- **PIX QR code generation** — produce a real EMV payload with 15-minute
  expiry (SEFAZ-SP already offers `integrador.sp.gov.br/pix-detran` for this).
- **LICENCIAMENTO debt type** — add the annual TRLAV (R$174.08) with the
  same 0.33%/day + 20% cap rule as IPVA. Zero changes to Simulator or
  Calculator are needed; only a new `InterestStrategy` and `DebtType` constant.
- **Circuit breaker** — wrap each provider with `sony/gobreaker` to avoid
  hammering a failing upstream.
- **Response caching** — short-TTL (30 s) Redis cache keyed by plate to
  absorb repeated requests without hitting providers.
- **OpenTelemetry tracing** — add span context through handler → use case →
  provider for end-to-end latency visibility.
- **Real Selic-based interest** — replace simplified daily rates with the
  monthly SEFAZ-SP Selic table for production-accurate calculations.

---

## Domain context

This service simulates a real Brazilian vehicle debt ecosystem:

| Provider in test | Real-world equivalent |
|------------------|-----------------------|
| Provider A (JSON) | SEFAZ-SP API via `infosimples.com` or `integrador.sp.gov.br` |
| Provider B (XML) | Legacy DETRAN-SP SOAP/REST integration |

Real debt types in SP: **IPVA** (annual tax), **MULTA** (traffic fine),
**LICENCIAMENTO/TRLAV** (R$174.08 annual CRLV fee), **DPVAT** (suspended 2020).

PIX payment for vehicle debts is already live in SP — SEFAZ-SP issues QR
codes with 15-minute validity via the dedicated `pix-detran` API.
