package service_test

import (
	"math"
	"testing"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
)

func TestSimulator_Simulate_PIX(t *testing.T) {
	sim := service.NewSimulator()
	debts := updatedDebts()
	result := sim.Simulate("ABC1234", debts)

	totalOpt := result.Payment.Options[0]
	if totalOpt.Type != "TOTAL" {
		t.Fatalf("first option type = %q, want TOTAL", totalOpt.Type)
	}

	// PIX = base * 0.92
	wantPix := round2(totalOpt.BaseAmount * 0.92)
	if totalOpt.Pix.TotalWithDiscount != wantPix {
		t.Errorf("PIX total = %.2f, want %.2f", totalOpt.Pix.TotalWithDiscount, wantPix)
	}
}

func TestSimulator_Simulate_CardFormula(t *testing.T) {
	sim := service.NewSimulator()
	debts := updatedDebts()
	result := sim.Simulate("ABC1234", debts)

	totalOpt := result.Payment.Options[0]
	base := totalOpt.BaseAmount

	installments := totalOpt.Card.Installments
	if len(installments) != 3 {
		t.Fatalf("expected 3 installment options, got %d", len(installments))
	}

	// Verify each installment against the literal spec formula: base * (1.025)^n / n
	for _, inst := range installments {
		n := inst.Quantity
		want := round2(base * math.Pow(1.025, float64(n)) / float64(n))
		if inst.InstallmentAmt != want {
			t.Errorf("installment %dx = %.2f, want %.2f", n, inst.InstallmentAmt, want)
		}
	}
}

func TestSimulator_Simulate_PaymentOptions(t *testing.T) {
	sim := service.NewSimulator()
	debts := updatedDebts()
	result := sim.Simulate("ABC1234", debts)

	// Expect: TOTAL, SOMENTE_IPVA, SOMENTE_MULTA
	if len(result.Payment.Options) != 3 {
		t.Fatalf("expected 3 payment options, got %d", len(result.Payment.Options))
	}

	types := make(map[string]float64)
	for _, o := range result.Payment.Options {
		types[o.Type] = o.BaseAmount
	}

	if _, ok := types["TOTAL"]; !ok {
		t.Error("missing TOTAL option")
	}
	if _, ok := types["SOMENTE_IPVA"]; !ok {
		t.Error("missing SOMENTE_IPVA option")
	}
	if _, ok := types["SOMENTE_MULTA"]; !ok {
		t.Error("missing SOMENTE_MULTA option")
	}
}

func TestSimulator_Simulate_Summary(t *testing.T) {
	sim := service.NewSimulator()
	debts := updatedDebts()
	result := sim.Simulate("ABC1234", debts)

	wantOriginal := round2(1500.00 + 300.50)
	wantUpdated := round2(1800.00 + 543.91)

	if result.Summary.TotalOriginal != wantOriginal {
		t.Errorf("TotalOriginal = %.2f, want %.2f", result.Summary.TotalOriginal, wantOriginal)
	}
	if result.Summary.TotalUpdated != wantUpdated {
		t.Errorf("TotalUpdated = %.2f, want %.2f", result.Summary.TotalUpdated, wantUpdated)
	}
}

func TestSimulator_Simulate_ExtensibleNewType(t *testing.T) {
	// Adding a new debt type (LICENCIAMENTO) should automatically produce
	// a SOMENTE_LICENCIAMENTO option without any code changes to Simulator.
	sim := service.NewSimulator()

	const debtTypeLicenciamento entity.DebtType = "LICENCIAMENTO"
	debts := []entity.UpdatedDebt{
		{
			Debt:          entity.Debt{Type: entity.DebtTypeIPVA, Amount: 1000, DueDate: time.Now()},
			UpdatedAmount: 1100,
		},
		{
			Debt:          entity.Debt{Type: debtTypeLicenciamento, Amount: 174.08, DueDate: time.Now()},
			UpdatedAmount: 180.00,
		},
	}

	result := sim.Simulate("XYZ9999", debts)

	found := false
	for _, o := range result.Payment.Options {
		if o.Type == "SOMENTE_LICENCIAMENTO" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SOMENTE_LICENCIAMENTO option for new debt type, not found")
	}
}

// updatedDebts returns the canonical test fixture matching the spec's provider data,
// with mathematically correct interest applied (see SPEC-AMBI-02).
func updatedDebts() []entity.UpdatedDebt {
	calc := service.NewCalculator(referenceDate)
	return calc.Apply([]entity.Debt{
		{
			Type:    entity.DebtTypeIPVA,
			Amount:  1500.00,
			DueDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			Type:    entity.DebtTypeMULTA,
			Amount:  300.50,
			DueDate: time.Date(2024, 2, 19, 0, 0, 0, 0, time.UTC),
		},
	})
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
