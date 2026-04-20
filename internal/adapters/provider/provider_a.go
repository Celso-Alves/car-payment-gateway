package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
)

// providerAResponse is the JSON wire format for Provider A.
// This mirrors the SEFAZ-SP style API response.
type providerAResponse struct {
	Vehicle string        `json:"vehicle"`
	Debts   []providerADebt `json:"debts"`
}

type providerADebt struct {
	Type    string  `json:"type"`
	Amount  float64 `json:"amount"`
	DueDate string  `json:"due_date"` // format: YYYY-MM-DD
}

// ProviderA is the Adapter (SPEC-001) for Provider A's JSON format.
// In production this would make an HTTP call to the SEFAZ-SP API;
// here it serves a deterministic in-memory payload to isolate the
// architecture from external dependencies.
type ProviderA struct {
	// payload holds the raw JSON bytes. Injected at construction so the
	// adapter can be tested with arbitrary provider responses.
	payload []byte
}

// NewProviderA constructs a ProviderA with a static JSON payload that
// mirrors the spec's Provider A example exactly.
func NewProviderA(plate string) *ProviderA {
	raw := fmt.Sprintf(`{
		"vehicle": %q,
		"debts": [
			{"type": "IPVA",  "amount": 1500.00, "due_date": "2024-01-10"},
			{"type": "MULTA", "amount": 300.50,  "due_date": "2024-02-19"}
		]
	}`, plate)
	return &ProviderA{payload: []byte(raw)}
}

func (p *ProviderA) Name() string { return "ProviderA-JSON" }

// FetchDebts parses the JSON payload and normalises it into the canonical
// []entity.Debt model (SPEC-002).
func (p *ProviderA) FetchDebts(ctx context.Context, _ string) ([]entity.Debt, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%s: context cancelled: %w", p.Name(), err)
	}

	var resp providerAResponse
	if err := json.Unmarshal(p.payload, &resp); err != nil {
		return nil, fmt.Errorf("%s: json unmarshal: %w", p.Name(), err)
	}

	debts := make([]entity.Debt, 0, len(resp.Debts))
	for _, d := range resp.Debts {
		dueDate, err := time.Parse("2006-01-02", d.DueDate)
		if err != nil {
			return nil, fmt.Errorf("%s: parse due_date %q: %w", p.Name(), d.DueDate, err)
		}
		debts = append(debts, entity.Debt{
			Type:    entity.DebtType(d.Type),
			Amount:  d.Amount,
			DueDate: dueDate,
		})
	}
	return debts, nil
}
