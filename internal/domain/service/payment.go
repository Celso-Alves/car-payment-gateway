package service

import (
	"fmt"
	"math"
	"sort"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
)

// installmentCounts are the credit card installment options offered (SPEC-004).
var installmentCounts = []int{1, 6, 12}

// pixDiscountRate is the PIX discount applied over the updated amount (SPEC-004).
const pixDiscountRate = 0.08

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
			TotalOriginal: round2(totalOriginal),
			TotalUpdated:  round2(totalUpdated),
		},
	}
	result.Payment.Options = options
	return result
}

// buildOption constructs a PaymentOption for the given label and base amount.
func buildOption(label string, base float64) entity.PaymentOption {
	base = round2(base)
	return entity.PaymentOption{
		Type:       label,
		BaseAmount: base,
		Pix:        buildPix(base),
		Card:       buildCard(base),
	}
}

// buildPix applies the 8% PIX discount (SPEC-004).
func buildPix(base float64) entity.PixOption {
	return entity.PixOption{
		TotalWithDiscount: round2(base * (1 - pixDiscountRate)),
	}
}

// buildCard computes installment amounts using the formula specified in the
// test document (SPEC-004):
//
//	installment = valor_total * (1.025)^n / n
//
// SPEC-AMBI-03: This formula does not match the numeric examples in the
// spec's expected output (e.g. 6x on 2355.93 → formula gives 455.36,
// spec shows 417.81). We implement the literal written formula and document
// this discrepancy in the README. The correct financial formula would be
// the Price/PMT amortization: PV * i / (1 - (1+i)^-n).
//
// SPEC-AMBI-05: The spec example shows SOMENTE_IPVA with only 1 installment.
// Since no written rule restricts installments by debt type, we offer
// 1x/6x/12x consistently for all options.
func buildCard(base float64) entity.CardOption {
	installments := make([]entity.Installment, 0, len(installmentCounts))
	for _, n := range installmentCounts {
		amount := base * math.Pow(1.025, float64(n)) / float64(n)
		installments = append(installments, entity.Installment{
			Quantity:       n,
			InstallmentAmt: round2(amount),
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

func sumOriginal(debts []entity.UpdatedDebt) float64 {
	var total float64
	for _, d := range debts {
		total += d.Amount
	}
	return total
}

func sumUpdated(debts []entity.UpdatedDebt) float64 {
	var total float64
	for _, d := range debts {
		total += d.UpdatedAmount
	}
	return total
}
