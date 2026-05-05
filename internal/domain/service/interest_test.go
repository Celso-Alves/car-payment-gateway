package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
	"github.com/celsoadsjr/car-payment-gateway/pkg/logger"
	"github.com/shopspring/decimal"
)

// referenceDate is the fixed test date specified in the document.
var referenceDate = time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)

func TestCalculator_Apply(t *testing.T) {
	log := logger.NewDiscard()
	calc := service.NewCalculator(referenceDate, log)

	tests := []struct {
		name            string
		debt            entity.Debt
		wantUpdated     decimal.Decimal
		wantDaysOverdue int
	}{
		{
			name: "IPVA: 121 days overdue, interest capped at 20%",
			debt: entity.Debt{
				Type:    entity.DebtTypeIPVA,
				Amount:  decimal.RequireFromString("1500.00"),
				DueDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			},
			wantUpdated:     decimal.RequireFromString("1800.00"),
			wantDaysOverdue: 121,
		},
		{
			name: "MULTA: 85 days overdue, no cap (HomeTest.pdf spec)",
			debt: entity.Debt{
				Type:    entity.DebtTypeMULTA,
				Amount:  decimal.RequireFromString("300.50"),
				DueDate: time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
			},
			wantUpdated:     decimal.RequireFromString("555.93"),
			wantDaysOverdue: 85,
		},
		{
			name: "IPVA: interest below cap, not capped",
			debt: entity.Debt{
				Type:    entity.DebtTypeIPVA,
				Amount:  decimal.RequireFromString("1000.00"),
				DueDate: time.Date(2024, 4, 10, 0, 0, 0, 0, time.UTC),
			},
			wantUpdated:     decimal.RequireFromString("1099.00"),
			wantDaysOverdue: 30,
		},
		{
			name: "IPVA: exactly at 20% cap boundary",
			debt: entity.Debt{
				Type:    entity.DebtTypeIPVA,
				Amount:  decimal.RequireFromString("1000.00"),
				DueDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			},
			wantUpdated:     decimal.RequireFromString("1200.00"),
			wantDaysOverdue: 121,
		},
		{
			name: "MULTA: 1 day overdue",
			debt: entity.Debt{
				Type:    entity.DebtTypeMULTA,
				Amount:  decimal.RequireFromString("100.00"),
				DueDate: time.Date(2024, 5, 9, 0, 0, 0, 0, time.UTC),
			},
			wantUpdated:     decimal.RequireFromString("101.00"),
			wantDaysOverdue: 1,
		},
		{
			name: "Not overdue: due date equals reference date",
			debt: entity.Debt{
				Type:    entity.DebtTypeIPVA,
				Amount:  decimal.RequireFromString("500.00"),
				DueDate: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			},
			wantUpdated:     decimal.RequireFromString("500.00"),
			wantDaysOverdue: 0,
		},
		{
			name: "Not overdue: due date in the future",
			debt: entity.Debt{
				Type:    entity.DebtTypeMULTA,
				Amount:  decimal.RequireFromString("200.00"),
				DueDate: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			},
			wantUpdated:     decimal.RequireFromString("200.00"),
			wantDaysOverdue: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := calc.Apply([]entity.Debt{tt.debt})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			got := results[0]

			if !got.UpdatedAmount.Equal(tt.wantUpdated) {
				t.Errorf("UpdatedAmount = %s, want %s", got.UpdatedAmount, tt.wantUpdated)
			}
			if got.DaysOverdue != tt.wantDaysOverdue {
				t.Errorf("DaysOverdue = %d, want %d", got.DaysOverdue, tt.wantDaysOverdue)
			}
		})
	}
}

func TestCalculator_Apply_MultipleDebts(t *testing.T) {
	log := logger.NewDiscard()
	calc := service.NewCalculator(referenceDate, log)

	debts := []entity.Debt{
		{
			Type:    entity.DebtTypeIPVA,
			Amount:  decimal.RequireFromString("1500.00"),
			DueDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			Type:    entity.DebtTypeMULTA,
			Amount:  decimal.RequireFromString("300.50"),
			DueDate: time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	results, err := calc.Apply(debts)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if !results[0].UpdatedAmount.Equal(decimal.RequireFromString("1800.00")) {
		t.Errorf("IPVA updated = %s, want 1800.00", results[0].UpdatedAmount)
	}
	if !results[1].UpdatedAmount.Equal(decimal.RequireFromString("555.93")) {
		t.Errorf("MULTA updated = %s, want 555.93", results[1].UpdatedAmount)
	}
}

func TestCalculator_Apply_UnknownType_AllUnknown(t *testing.T) {
	log := logger.NewDiscard()
	calc := service.NewCalculator(referenceDate, log)

	debts := []entity.Debt{
		{
			Type:    entity.DebtType("LICENCIAMENTO"),
			Amount:  decimal.RequireFromString("100.00"),
			DueDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	_, err := calc.Apply(debts)
	if err == nil {
		t.Fatal("expected error when all debt types are unknown")
	}
	if !errors.Is(err, service.ErrAllDebtsUnknownType) {
		t.Fatalf("error = %v, want ErrAllDebtsUnknownType", err)
	}
}

func TestCalculator_Apply_UnknownType_Partial(t *testing.T) {
	log := logger.NewDiscard()
	calc := service.NewCalculator(referenceDate, log)

	debts := []entity.Debt{
		{
			Type:    entity.DebtTypeIPVA,
			Amount:  decimal.RequireFromString("1000.00"),
			DueDate: time.Date(2024, 4, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			Type:    entity.DebtType("LICENCIAMENTO"),
			Amount:  decimal.RequireFromString("50.00"),
			DueDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	results, err := calc.Apply(debts)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Unprocessed {
		t.Error("IPVA should be processed")
	}
	if !results[1].Unprocessed {
		t.Error("LICENCIAMENTO should be unprocessed")
	}
	if !results[1].UpdatedAmount.Equal(decimal.RequireFromString("50.00")) {
		t.Errorf("unknown type amount = %s, want 50.00", results[1].UpdatedAmount)
	}
}
