package service_test

import (
	"testing"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
)

// referenceDate is the fixed test date specified in the document.
var referenceDate = time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)

func TestCalculator_Apply(t *testing.T) {
	calc := service.NewCalculator(referenceDate)

	tests := []struct {
		name            string
		debt            entity.Debt
		wantUpdated     float64
		wantDaysOverdue int
	}{
		{
			name: "IPVA: 121 days overdue, interest capped at 20%",
			debt: entity.Debt{
				Type:    entity.DebtTypeIPVA,
				Amount:  1500.00,
				DueDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			},
			// 1500 * 0.0033 * 121 = 599.55 → exceeds 20% cap (300) → 1800.00
			wantUpdated:     1800.00,
			wantDaysOverdue: 121,
		},
		{
			// SPEC-AMBI-02: The test document states "85 days" and 555.93 for this case.
			// Correct UTC day-diff from 2024-02-19 to 2024-05-10 is 81 days.
			// We implement mathematically correct date subtraction (81 days → 543.91)
			// and document this discrepancy in the README.
			name: "MULTA: 81 days overdue, no cap (spec says 85 — see SPEC-AMBI-02)",
			debt: entity.Debt{
				Type:    entity.DebtTypeMULTA,
				Amount:  300.50,
				DueDate: time.Date(2024, 2, 19, 0, 0, 0, 0, time.UTC),
			},
			// 300.50 * 0.01 * 81 = 243.405 → 300.50 + 243.41 = 543.91
			wantUpdated:     543.91,
			wantDaysOverdue: 81,
		},
		{
			name: "IPVA: interest below cap, not capped",
			debt: entity.Debt{
				Type:    entity.DebtTypeIPVA,
				Amount:  1000.00,
				DueDate: time.Date(2024, 4, 10, 0, 0, 0, 0, time.UTC),
			},
			// 30 days * 0.0033 = 0.099 → 99.00 interest, cap = 200 → not capped
			wantUpdated:     1099.00,
			wantDaysOverdue: 30,
		},
		{
			name: "IPVA: exactly at 20% cap boundary",
			debt: entity.Debt{
				Type:    entity.DebtTypeIPVA,
				Amount:  1000.00,
				DueDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			},
			// 121 days * 0.0033 = 0.3993 → 399.30 > cap 200 → total 1200.00
			wantUpdated:     1200.00,
			wantDaysOverdue: 121,
		},
		{
			name: "MULTA: 1 day overdue",
			debt: entity.Debt{
				Type:    entity.DebtTypeMULTA,
				Amount:  100.00,
				DueDate: time.Date(2024, 5, 9, 0, 0, 0, 0, time.UTC),
			},
			// 100 * 0.01 * 1 = 1.00 → total 101.00
			wantUpdated:     101.00,
			wantDaysOverdue: 1,
		},
		{
			name: "Not overdue: due date equals reference date",
			debt: entity.Debt{
				Type:    entity.DebtTypeIPVA,
				Amount:  500.00,
				DueDate: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			},
			wantUpdated:     500.00,
			wantDaysOverdue: 0,
		},
		{
			name: "Not overdue: due date in the future",
			debt: entity.Debt{
				Type:    entity.DebtTypeMULTA,
				Amount:  200.00,
				DueDate: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			},
			wantUpdated:     200.00,
			wantDaysOverdue: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := calc.Apply([]entity.Debt{tt.debt})
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			got := results[0]

			if got.UpdatedAmount != tt.wantUpdated {
				t.Errorf("UpdatedAmount = %.2f, want %.2f", got.UpdatedAmount, tt.wantUpdated)
			}
			if got.DaysOverdue != tt.wantDaysOverdue {
				t.Errorf("DaysOverdue = %d, want %d", got.DaysOverdue, tt.wantDaysOverdue)
			}
		})
	}
}

func TestCalculator_Apply_MultipleDebts(t *testing.T) {
	calc := service.NewCalculator(referenceDate)

	debts := []entity.Debt{
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
	}

	results := calc.Apply(debts)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].UpdatedAmount != 1800.00 {
		t.Errorf("IPVA updated = %.2f, want 1800.00", results[0].UpdatedAmount)
	}
	// SPEC-AMBI-02: correct calc gives 543.91 (81 days), spec example shows 555.93 (85 days).
	if results[1].UpdatedAmount != 543.91 {
		t.Errorf("MULTA updated = %.2f, want 543.91", results[1].UpdatedAmount)
	}
}
