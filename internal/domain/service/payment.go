package service

import (
	"fmt"
	"sort"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/shopspring/decimal"
)

var (
	pixDiscountRate = decimal.RequireFromString("0.08")
	onePoint025     = decimal.RequireFromString("1.025")
)

// Simulator generates all payment options for a set of updated debts (SPEC-007).
// It groups debts by DebtType and produces:
//   - One TOTAL option (all debts combined)
//   - One SOMENTE_<TYPE> option per distinct debt type
//
// This design means that adding a new debt type (e.g. LICENCIAMENTO) automatically
// produces a new partial payment option with zero changes to this code.
type Simulator struct{}

// NewSimulator creates a ready-to-use Simulator.
func NewSimulator() *Simulator {
	return &Simulator{}
}

// Simulate builds the full ConsultResult from a plate and its updated debts.
func (s *Simulator) Simulate(plate string, debts []entity.UpdatedDebt) entity.ConsultResult {
	totalOriginal := sumOriginal(debts)
	totalUpdated := sumUpdated(debts)

	byType := groupByType(debts)

	// Collect sorted type keys so output order is deterministic.
	types := make([]entity.DebtType, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })

	options := make([]entity.PaymentOption, 0, 1+len(byType))

	// TOTAL option always comes first.
	options = append(options, buildOption("TOTAL", totalUpdated))

	// One partial option per debt type.
	for _, t := range types {
		groupTotal := sumUpdated(byType[t])
		label := fmt.Sprintf("SOMENTE_%s", t)
		options = append(options, buildOption(label, groupTotal))
	}

	result := entity.ConsultResult{
		Plate: plate,
		Summary: entity.PaymentSummary{
			TotalOriginal: totalOriginal.Round(2),
			TotalUpdated:  totalUpdated.Round(2),
		},
	}
	result.Payment.Options = options
	return result
}

// buildOption constructs a PaymentOption for the given label and base amount.
func buildOption(label string, base decimal.Decimal) entity.PaymentOption {
	base = base.Round(2)
	return entity.PaymentOption{
		Type:       label,
		BaseAmount: base,
		Pix:        buildPix(base),
		Card:       buildCard(base),
	}
}

// buildPix applies the 8% PIX discount (SPEC-004).
func buildPix(base decimal.Decimal) entity.PixOption {
	one := decimal.NewFromInt(1)
	factor := one.Sub(pixDiscountRate)
	return entity.PixOption{
		TotalWithDiscount: base.Mul(factor).Round(2),
	}
}

// buildCard computes installment amounts using the formula specified in the
// test document (SPEC-004):
//
//	installment = valor_total * (1.025)^n / n
//
// SPEC-AMBI-03: This formula does not match amortization (Price/PMT). We
// implement the literal written formula. The total with interest is rounded
// first; each of the first (n-1) parcels is round(base×1.025^n/n); the last
// parcel absorbs the remainder so the sum matches the rounded total exactly.
// The API exposes a single valor_parcela equal to the recurring parcel (the
// first n-1); the final payment may differ by cents in a real slip.
func buildCard(base decimal.Decimal) entity.CardOption {
	installmentCounts := []int{1, 6, 12}
	installments := make([]entity.Installment, 0, len(installmentCounts))

	for _, n := range installmentCounts {
		nDec := decimal.NewFromInt(int64(n))
		pow := onePoint025.Pow(nDec)
		target := base.Mul(pow).Round(2)

		var amount decimal.Decimal
		if n == 1 {
			amount = target
		} else {
			rawEach := base.Mul(pow).Div(nDec)
			each := rawEach.Round(2)
			amount = each
		}

		installments = append(installments, entity.Installment{
			Quantity: n,
			Amount:   amount,
		})
	}

	return entity.CardOption{Installments: installments}
}

// groupByType partitions updated debts into a map keyed by DebtType.
func groupByType(debts []entity.UpdatedDebt) map[entity.DebtType][]entity.UpdatedDebt {
	m := make(map[entity.DebtType][]entity.UpdatedDebt)
	for _, d := range debts {
		m[d.Type] = append(m[d.Type], d)
	}
	return m
}

func sumOriginal(debts []entity.UpdatedDebt) decimal.Decimal {
	total := decimal.Zero
	for _, d := range debts {
		total = total.Add(d.Amount)
	}
	return total
}

func sumUpdated(debts []entity.UpdatedDebt) decimal.Decimal {
	total := decimal.Zero
	for _, d := range debts {
		total = total.Add(d.UpdatedAmount)
	}
	return total
}
