package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/shopspring/decimal"
)

// providerAResponse is the JSON wire format for Provider A.
// This mirrors the SEFAZ-SP style API response.
type providerAResponse struct {
	Vehicle string          `json:"vehicle"`
	Debts   []providerADebt `json:"debts"`
}

type providerADebt struct {
	Type    string          `json:"type"`
	Amount  decimal.Decimal `json:"amount"`
	DueDate string          `json:"due_date"` // format: YYYY-MM-DD
}

// ProviderA is the Adapter (SPEC-001) for Provider A's JSON format.
// In production this would make an HTTP call to the SEFAZ-SP API;
// here it serves a deterministic in-memory payload to isolate the
// architecture from external dependencies.
type ProviderA struct{}

// NewProviderA constructs a ProviderA adapter.
func NewProviderA() *ProviderA {
	return &ProviderA{}
}

func (p *ProviderA) Name() string { return "ProviderA-JSON" }

// FetchDebts builds a deterministic JSON payload for the given plate and
// normalises it into the canonical []entity.Debt model (SPEC-002).
func (p *ProviderA) FetchDebts(ctx context.Context, plate string) ([]entity.Debt, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%s: context cancelled: %w", p.Name(), err)
	}

	raw := fmt.Sprintf(`{
		"vehicle": %q,
		"debts": [
			{"type": "IPVA",  "amount": 1500.00, "due_date": "2024-01-10"},
			{"type": "MULTA", "amount": 300.50,  "due_date": "2024-02-15"}
		]
	}`, plate)

	var resp providerAResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
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
