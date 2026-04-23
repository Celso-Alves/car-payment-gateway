package service_test

import (
	"testing"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
	"github.com/celsoadsjr/car-payment-gateway/pkg/logger"
	"github.com/shopspring/decimal"
)

func TestSimulator_Simulate_PIX(t *testing.T) {
	sim := service.NewSimulator()
	debts := updatedDebts(t)
	result := sim.Simulate("ABC1234", debts)

	totalOpt := result.Payment.Options[0]
	if totalOpt.Type != "TOTAL" {
		t.Fatalf("first option type = %q, want TOTAL", totalOpt.Type)
	}

	wantPix := totalOpt.BaseAmount.Mul(decimal.RequireFromString("0.92")).Round(2)
	if !totalOpt.Pix.TotalWithDiscount.Equal(wantPix) {
		t.Errorf("PIX total = %s, want %s", totalOpt.Pix.TotalWithDiscount, wantPix)
	}
}

func TestSimulator_Simulate_CardFormula(t *testing.T) {
	sim := service.NewSimulator()
	debts := updatedDebts(t)
	result := sim.Simulate("ABC1234", debts)

	totalOpt := result.Payment.Options[0]
	base := totalOpt.BaseAmount

	installments := totalOpt.Card.Installments
	if len(installments) != 3 {
		t.Fatalf("expected 3 installment options, got %d", len(installments))
	}

	one025 := decimal.RequireFromString("1.025")
	for _, inst := range installments {
		n := inst.Quantity
		nDec := decimal.NewFromInt(int64(n))
		pow := one025.Pow(nDec)
		target := base.Mul(pow).Round(2)
		var want decimal.Decimal
		if n == 1 {
			want = target
		} else {
			want = base.Mul(pow).Div(nDec).Round(2)
		}
		if !inst.Amount.Equal(want) {
			t.Errorf("installment %dx = %s, want %s", n, inst.Amount, want)
		}
		if n > 1 {
			last := target.Sub(inst.Amount.Mul(nDec.Sub(decimal.NewFromInt(1)))).Round(2)
			sum := inst.Amount.Mul(nDec.Sub(decimal.NewFromInt(1))).Add(last)
			if !sum.Equal(target) {
				t.Errorf("installment %dx: sum %s != target %s (last=%s)", n, sum, target, last)
			}
		}
	}
}

func TestSimulator_Simulate_PaymentOptions(t *testing.T) {
	sim := service.NewSimulator()
	debts := updatedDebts(t)
	result := sim.Simulate("ABC1234", debts)

	if len(result.Payment.Options) != 3 {
		t.Fatalf("expected 3 payment options, got %d", len(result.Payment.Options))
	}

	types := make(map[string]bool)
	for _, o := range result.Payment.Options {
		types[o.Type] = true
	}

	for _, r := range []string{"TOTAL", "SOMENTE_IPVA", "SOMENTE_MULTA"} {
		if !types[r] {
			t.Errorf("missing payment option %q", r)
		}
	}
}

func TestSimulator_Simulate_Summary(t *testing.T) {
	sim := service.NewSimulator()
	debts := updatedDebts(t)
	result := sim.Simulate("ABC1234", debts)

	wantOriginal := decimal.RequireFromString("1500.00").Add(decimal.RequireFromString("300.50")).Round(2)
	wantUpdated := decimal.RequireFromString("1800.00").Add(decimal.RequireFromString("543.91")).Round(2)

	if !result.Summary.TotalOriginal.Equal(wantOriginal) {
		t.Errorf("TotalOriginal = %s, want %s", result.Summary.TotalOriginal, wantOriginal)
	}
	if !result.Summary.TotalUpdated.Equal(wantUpdated) {
		t.Errorf("TotalUpdated = %s, want %s", result.Summary.TotalUpdated, wantUpdated)
	}
}

func TestSimulator_Simulate_ExtensibleNewType(t *testing.T) {
	sim := service.NewSimulator()

	const debtTypeLicenciamento entity.DebtType = "LICENCIAMENTO"
	debts := []entity.UpdatedDebt{
		{
			Debt:          entity.Debt{Type: entity.DebtTypeIPVA, Amount: decimal.RequireFromString("1000"), DueDate: time.Date(2024, 4, 10, 0, 0, 0, 0, time.UTC)},
			UpdatedAmount: decimal.RequireFromString("1099.00"),
		},
		{
			Debt:          entity.Debt{Type: debtTypeLicenciamento, Amount: decimal.RequireFromString("174.08"), DueDate: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)},
			UpdatedAmount: decimal.RequireFromString("180.00"),
			Unprocessed:   true,
		},
	}

	result := sim.Simulate("XYZ9999", debts)

	var somenteLic *entity.PaymentOption
	for i := range result.Payment.Options {
		if result.Payment.Options[i].Type == "SOMENTE_LICENCIAMENTO" {
			somenteLic = &result.Payment.Options[i]
			break
		}
	}
	if somenteLic == nil {
		t.Fatal("expected SOMENTE_LICENCIAMENTO option")
	}

	base := somenteLic.BaseAmount
	wantPix := base.Mul(decimal.RequireFromString("0.92")).Round(2)
	if !somenteLic.Pix.TotalWithDiscount.Equal(wantPix) {
		t.Errorf("SOMENTE_LICENCIAMENTO PIX = %s, want %s", somenteLic.Pix.TotalWithDiscount, wantPix)
	}

	one025 := decimal.RequireFromString("1.025")
	for _, inst := range somenteLic.Card.Installments {
		n := inst.Quantity
		nDec := decimal.NewFromInt(int64(n))
		pow := one025.Pow(nDec)
		target := base.Mul(pow).Round(2)
		var want decimal.Decimal
		if n == 1 {
			want = target
		} else {
			want = base.Mul(pow).Div(nDec).Round(2)
		}
		if !inst.Amount.Equal(want) {
			t.Errorf("SOMENTE_LICENCIAMENTO parcela %dx = %s, want %s", n, inst.Amount, want)
		}
		if n > 1 {
			last := target.Sub(inst.Amount.Mul(nDec.Sub(decimal.NewFromInt(1)))).Round(2)
			sum := inst.Amount.Mul(nDec.Sub(decimal.NewFromInt(1))).Add(last)
			if !sum.Equal(target) {
				t.Errorf("SOMENTE_LICENCIAMENTO %dx sum %s != target %s", n, sum, target)
			}
		}
	}
}

func updatedDebts(t *testing.T) []entity.UpdatedDebt {
	t.Helper()
	log := logger.NewDiscard()
	calc := service.NewCalculator(referenceDate, log)
	out, err := calc.Apply([]entity.Debt{
		{
			Type:    entity.DebtTypeIPVA,
			Amount:  decimal.RequireFromString("1500.00"),
			DueDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			Type:    entity.DebtTypeMULTA,
			Amount:  decimal.RequireFromString("300.50"),
			DueDate: time.Date(2024, 2, 19, 0, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return out
}
