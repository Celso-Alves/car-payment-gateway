package provider

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
)

// providerBResponse is the XML wire format for Provider B.
// This mirrors a legacy DETRAN-SP style SOAP/REST response.
type providerBResponse struct {
	XMLName xml.Name      `xml:"response"`
	Plate   string        `xml:"plate"`
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
type ProviderB struct {
	payload []byte
}

// NewProviderB constructs a ProviderB with a static XML payload that
// mirrors the spec's Provider B example exactly.
func NewProviderB(plate string) *ProviderB {
	raw := fmt.Sprintf(`<response>
	<plate>%s</plate>
	<debts>
		<debt><category>IPVA</category><value>1500.00</value><expiration>2024-01-10</expiration></debt>
		<debt><category>MULTA</category><value>300.50</value><expiration>2024-02-19</expiration></debt>
	</debts>
</response>`, plate)
	return &ProviderB{payload: []byte(raw)}
}

func (p *ProviderB) Name() string { return "ProviderB-XML" }

// FetchDebts parses the XML payload and normalises it into the canonical
// []entity.Debt model (SPEC-002).
func (p *ProviderB) FetchDebts(ctx context.Context, _ string) ([]entity.Debt, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%s: context cancelled: %w", p.Name(), err)
	}

	var resp providerBResponse
	if err := xml.Unmarshal(p.payload, &resp); err != nil {
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
			Amount:  d.Value,
			DueDate: dueDate,
		})
	}
	return debts, nil
}
