package provider

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/shopspring/decimal"
)

// providerBResponse is the XML wire format for Provider B.
// This mirrors a legacy DETRAN-SP style SOAP/REST response.
type providerBResponse struct {
	XMLName xml.Name        `xml:"response"`
	Plate   string          `xml:"plate"`
	Debts   []providerBDebt `xml:"debts>debt"`
}

type providerBDebt struct {
	Category   string  `xml:"category"`
	Value      float64 `xml:"value"`
	Expiration string  `xml:"expiration"` // format: YYYY-MM-DD
}

// ProviderB is the Adapter (SPEC-001) for Provider B's XML format.
// Despite returning the same logical data as ProviderA, it uses a
// completely different wire format (XML vs JSON) and different field names
// (category/value/expiration vs type/amount/due_date). The adapter
// normalises both into the same canonical entity.Debt.
type ProviderB struct{}

// NewProviderB constructs a ProviderB adapter.
func NewProviderB() *ProviderB {
	return &ProviderB{}
}

func (p *ProviderB) Name() string { return "ProviderB-XML" }

// FetchDebts builds a deterministic XML payload for the given plate and
// normalises it into the canonical []entity.Debt model (SPEC-002).
func (p *ProviderB) FetchDebts(ctx context.Context, plate string) ([]entity.Debt, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%s: context cancelled: %w", p.Name(), err)
	}

	raw := fmt.Sprintf(`<response>
	<plate>%s</plate>
	<debts>
		<debt><category>IPVA</category><value>1500.00</value><expiration>2024-01-10</expiration></debt>
		<debt><category>MULTA</category><value>300.50</value><expiration>2024-02-15</expiration></debt>
	</debts>
</response>`, plate)

	var resp providerBResponse
	if err := xml.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("%s: xml unmarshal: %w", p.Name(), err)
	}

	debts := make([]entity.Debt, 0, len(resp.Debts))
	for _, d := range resp.Debts {
		dueDate, err := time.Parse("2006-01-02", d.Expiration)
		if err != nil {
			return nil, fmt.Errorf("%s: parse expiration %q: %w", p.Name(), d.Expiration, err)
		}
		debts = append(debts, entity.Debt{
			Type:    entity.DebtType(d.Category),
			Amount:  decimal.NewFromFloat(d.Value),
			DueDate: dueDate,
		})
	}
	return debts, nil
}
