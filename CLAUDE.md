# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

```bash
make run              # Start server on :3000 (respects PORT, ADDR, REFERENCE_DATE from .env)
make build            # Build production binary to bin/
make test             # Run all tests once, no verbosity
make test-verbose     # All tests with detailed output
make test-race        # Tests with Go race detector (for concurrency changes)
make coverage         # Show coverage report (coverage.out + terminal summary)
make lint             # go vet ./...
make docker-run       # Build image and run server in container on :3000
make demo-fallback    # Run with ENABLE_MOCK_FAILING=true (provider timeout demo)
make demo-timeout     # Run with ENABLE_MOCK_SLOW=true (fallback chain demo)
```

## Project Context

**Service**: Vehicle debt consultation + payment simulation for São Paulo vehicle registry (DETRAN/SEFAZ-SP mock integration).

**Tech**: Go 1.24+, no framework (stdlib http, slog), monetary values use `shopspring/decimal` (not float64).

**Input/Output**: `POST /api/v1/debts { "placa": "ABC1234" }` → returns original + interest-updated debt summary, payment options (PIX 8% discount, credit card 1/6/12 installments).

## Architecture (Clean/Hexagonal)

Four concentric layers with strict dependency rules:

1. **Domain** (`internal/domain/`) — pure business logic, zero imports of HTTP or adapters
   - `entity/` — canonical types (`Debt`, `UpdatedDebt`, `PaymentSummary`, `Installment`)
   - `service/` — domain services: `Calculator` (interest by DebtType), `Simulator` (payment options)

2. **Application** (`internal/application/usecase/`) — orchestration layer
   - `ConsultDebts` — facade that chains: providers (fallback + retry) → Calculator → Simulator
   - Depends only on domain types/services and the `Provider` port; never concrete adapters

3. **Adapters** (`internal/adapters/`)
   - `provider/` — implements `Provider` port: `ProviderA` (JSON), `ProviderB` (XML), `MockFailing` (demo)
   - `httpapi/` — HTTP handler, request validation, response marshaling, recover middleware

4. **Composition** (`cmd/api/main.go`) — wiring, env vars, graceful shutdown, route registration

**Key Rule**: Dependencies point inward only. Domain cannot import adapters or HTTP. Application imports only ports (interfaces), not concrete implementations.

## Debt Calculation Rules (from README SPEC references)

- **IPVA** (annual tax): 0.33% per day, capped at 20% interest
- **MULTA** (fine): 1% per day, no cap
- **Interest**: simple daily compounding from due date to `REFERENCE_DATE` (env var, defaults to now)
- **Rounding**: `decimal.Round(2)` (half-up), serialized as JSON string, never float64
- **Days overdue**: correct UTC date subtraction (not spec's simplified counting)

## Environment Variables

```
PORT=3000                        # Server port
ADDR=                            # Full listen addr (overrides PORT)
REFERENCE_DATE=2024-05-10        # Fixes calculation date (YYYY-MM-DD); defaults to now
LOG_LEVEL=info                   # slog level (debug, info, warn, error)
ENABLE_MOCK_FAILING=true         # Prepend failing mock for fallback demo
ENABLE_MOCK_SLOW=true            # Prepend slow mock (timeout) for timeout demo
PROVIDER_MAX_ATTEMPTS=1          # Retries per provider
PROVIDER_RETRY_BACKOFF_MS=0      # Wait between retries
```

## Development Rules (from `.cursor/rules/`)

### Keep Architecture Boundaries
- Domain must stay pure: no HTTP, no adapters, no providers imported
- Application orchestrates via ports, never touches concrete adapters
- Adapters handle I/O translation; no business logic
- Composition only in `cmd/api/`

### When Adding Features
1. **New debt type** → add constant in `entity/`, new `InterestStrategy` in `service/interest.go`, unit test in `service/interest_test.go`
2. **New provider** → implement `provider.Provider` interface (see `ProviderA`, `ProviderB`), register in `cmd/api/main.go`
3. **HTTP change** → implement/adjust in `httpapi/handler.go` or `middleware.go`, wire in `main.go`, test in `handler_test.go`

### Error & Data Safety
- **Plate logging**: always use `pkg/logger.MaskPlate(plate)` — masks to `ABC-****`
- **Monetary values**: use `shopspring/decimal`, never `float64`
- **HTTP input validation**: `MaxBytesReader` (1 MiB), `DisallowUnknownFields`, strict `placa` regex `^[A-Z]{3}-?[0-9][A-Z0-9][0-9]{2}$`
- **Error mapping**: unknown debt types → 400, all providers unavailable → 503, internal errors → 500

### Fallback Behavior (SPEC-005)
- Try each provider in order (first success wins)
- Per-provider timeout: 3s default (`providerTimeout` in `ConsultDebts`)
- Per-provider retries: configurable via `PROVIDER_MAX_ATTEMPTS` and `PROVIDER_RETRY_BACKOFF_MS`
- If all fail → return `ErrAllProvidersFailed` (HTTP 503)

### Timeouts
- Per-provider call: `providerTimeout` (3s default)
- Request context: `requestTimeout` (10s default, set in `ConsultDebts`)
- Server read/write/idle: 15s/15s/60s (set in `http.Server` config in `main.go`)

## Testing Strategy

- **Domain logic** (`service/` tests): unit tests with fixed reference dates, test both rounding and edge cases
- **Use case** (`usecase/` tests): mock providers, test fallback chain and retry logic
- **HTTP** (`httpapi/` tests): test status codes, validation errors, full request/response cycle
- Always run `make test` before committing; use `make test-race` if concurrency touched

## Code Patterns

| Pattern | Where | Example |
|---------|-------|---------|
| **Strategy** | `Calculator` / `InterestStrategy` | Different interest rates per `DebtType` |
| **Adapter** | `ProviderA`, `ProviderB` | Each normalizes wire format → `entity.Debt` |
| **Facade** | `ConsultDebts` | Single entry point hiding provider chain + calculation |
| **Dependency Injection** | `main.go` | Manual wiring (no DI framework needed at this scale) |
| **Group-by map** | `Simulator` | Debts grouped by `DebtType` → auto-generates partial options |

## Quirks & Trade-offs

- **Day counting**: Uses mathematically correct UTC subtraction (81 days for Feb 19 → May 10, 2024), diverges from spec's simplified 85-day count. Both approaches documented in README.
- **Installment formula**: Implements literal spec formula `(1.025)^n / n`, not Price/PMT. Gap documented vs. spec examples.
- **Partial payments**: Always offer 1x/6x/12x consistently; spec only shows 1x for SOMENTE_IPVA but no rule stated. RFC area: see README "SPEC-AMBI-05".
- **REFERENCE_DATE**: Required for reproducible testing; see `ConsultDebts` setup in `main.go`.

## Future Improvements (from README)

- Real HTTP clients for providers (SEFAZ-SP, DETRAN-SP integration)
- Circuit breaker per provider
- Redis cache (30s TTL keyed by plate)
- OpenTelemetry propagation (main → usecase → provider spans)
- Selic-based interest rates (real legislation)
- QR code generation for PIX (EMV payload, 15-min validity)

## Files to Ignore

- `.env` (local secrets; committed is `.env-example`)
- `bin/` (build output)
- `coverage.out` (test reports)
- `.cursor/` (editor config)

## Key Takeaways

1. **Domain stays pure** — no HTTP, providers, or framework leakage. Test it independently.
2. **Fallback is first-class** — provider chain with timeout + retry is core behavior, not a side effect.
3. **Money is a string** — `shopspring/decimal` all the way, JSON serialized as string.
4. **Providers are pluggable** — new provider = one file + one line in `main.go`.
5. **Small, testable commits** — each behavior change should be independently unit-tested and runnable via `make test`.
