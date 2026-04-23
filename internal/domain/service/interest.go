// Package service contains the core domain services.
// Business rules live here and must never import adapter or HTTP packages.
package service

import (
	"errors"
	"log/slog"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/shopspring/decimal"
)

// ErrAllDebtsUnknownType is returned when every debt has a DebtType with no
// registered InterestStrategy (all would be passed through as Unprocessed).
var ErrAllDebtsUnknownType = errors.New("all debts have unknown types; no interest strategy applied")

// InterestStrategy is the Strategy interface (SPEC-003) that each debt
// type must implement. Adding a new debt type (e.g. LICENCIAMENTO) means
// creating a new strategy — no changes to Calculator or callers.
type InterestStrategy interface {
	// Calculate returns the interest-adjusted amount for the given debt,
	// using the provided reference date to compute days overdue.
	Calculate(debt entity.Debt, referenceDate time.Time) (updatedAmount decimal.Decimal, daysOverdue int)
}

var (
	dailyRateIPVA  = decimal.RequireFromString("0.0033")
	maxCapRateIPVA = decimal.RequireFromString("0.20")
	dailyRateMulta = decimal.RequireFromString("0.01")
)

// ipvaStrategy applies simple interest at 0.33%/day capped at 20% of
// the original value (SPEC-003).
type ipvaStrategy struct{}

func (s ipvaStrategy) Calculate(debt entity.Debt, ref time.Time) (decimal.Decimal, int) {
	days := daysOverdue(debt.DueDate, ref)
	if days <= 0 {
		return debt.Amount, 0
	}

	daysDec := decimal.NewFromInt(int64(days))
	interest := debt.Amount.Mul(dailyRateIPVA).Mul(daysDec)
	cap := debt.Amount.Mul(maxCapRateIPVA)
	if interest.GreaterThan(cap) {
		interest = cap
	}

	return debt.Amount.Add(interest).Round(2), days
}

// multaStrategy applies simple interest at 1%/day with no cap (SPEC-003).
type multaStrategy struct{}

func (s multaStrategy) Calculate(debt entity.Debt, ref time.Time) (decimal.Decimal, int) {
	days := daysOverdue(debt.DueDate, ref)
	if days <= 0 {
		return debt.Amount, 0
	}

	daysDec := decimal.NewFromInt(int64(days))
	interest := debt.Amount.Mul(dailyRateMulta).Mul(daysDec)
	return debt.Amount.Add(interest).Round(2), days
}

// Calculator applies the correct InterestStrategy for each debt type.
// The reference date is injected at construction time so tests can pin
// it to a fixed value (2024-05-10 as specified) and production can use
// time.Now().
type Calculator struct {
	referenceDate time.Time
	strategies    map[entity.DebtType]InterestStrategy
	log           *slog.Logger
}

// NewCalculator creates a Calculator with the given reference date and
// all known strategies pre-registered.
func NewCalculator(referenceDate time.Time, log *slog.Logger) *Calculator {
	if log == nil {
		log = slog.Default()
	}
	return &Calculator{
		referenceDate: referenceDate,
		strategies: map[entity.DebtType]InterestStrategy{
			entity.DebtTypeIPVA:  ipvaStrategy{},
			entity.DebtTypeMULTA: multaStrategy{},
		},
		log: log,
	}
}

// Apply returns an UpdatedDebt for every input debt, with the
// interest-adjusted amount computed by the appropriate strategy.
// Debts with unknown types are marked Unprocessed=true and passed through
// with the original amount. If every debt is unknown, returns ErrAllDebtsUnknownType.
func (c *Calculator) Apply(debts []entity.Debt) ([]entity.UpdatedDebt, error) {
	if len(debts) == 0 {
		return nil, nil
	}

	result := make([]entity.UpdatedDebt, 0, len(debts))
	unknownCount := 0

	for _, d := range debts {
		strategy, ok := c.strategies[d.Type]
		var updated decimal.Decimal
		var days int
		var unprocessed bool

		if ok {
			updated, days = strategy.Calculate(d, c.referenceDate)
		} else {
			unknownCount++
			unprocessed = true
			updated = d.Amount
			days = 0
			c.log.Warn("unknown debt type; passing amount without interest",
				slog.String("type", string(d.Type)),
			)
		}

		result = append(result, entity.UpdatedDebt{
			Debt:          d,
			UpdatedAmount: updated,
			DaysOverdue:   days,
			Unprocessed:   unprocessed,
		})
	}

	if unknownCount == len(debts) {
		return nil, ErrAllDebtsUnknownType
	}

	return result, nil
}

// daysOverdue computes the number of whole calendar days between dueDate and ref
// (date-only, midnight UTC). Example: 2024-02-19 → 2024-05-10 is 81 days.
// This differs from some spec examples that used 85 days for the same range
// (SPEC-AMBI-02); the implementation follows strict UTC date arithmetic.
func daysOverdue(dueDate, ref time.Time) int {
	due := truncateToDate(dueDate)
	reference := truncateToDate(ref)
	diff := reference.Sub(due)
	if diff <= 0 {
		return 0
	}
	return int(diff.Hours() / 24)
}

// truncateToDate normalises a time.Time to midnight UTC so that
// day arithmetic is timezone-independent.
func truncateToDate(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
