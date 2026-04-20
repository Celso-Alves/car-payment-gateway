package provider_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/adapters/provider"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
)

func TestProviderA_FetchDebts(t *testing.T) {
	p := provider.NewProviderA("ABC1234")

	debts, err := p.FetchDebts(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDebts(t, "ProviderA", debts)
}

func TestProviderB_FetchDebts(t *testing.T) {
	p := provider.NewProviderB("ABC1234")

	debts, err := p.FetchDebts(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDebts(t, "ProviderB", debts)
}

func TestProviderA_CancelledContext(t *testing.T) {
	p := provider.NewProviderA("ABC1234")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := p.FetchDebts(ctx, "ABC1234")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestProviderB_CancelledContext(t *testing.T) {
	p := provider.NewProviderB("ABC1234")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.FetchDebts(ctx, "ABC1234")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestMockFailing_AlwaysFails(t *testing.T) {
	p := &provider.MockFailing{}

	_, err := p.FetchDebts(context.Background(), "ABC1234")
	if err == nil {
		t.Fatal("expected MockFailing to return an error")
	}
	if !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Errorf("error = %v, want ErrProviderUnavailable", err)
	}
}

func TestMockFailing_SimulateTimeout(t *testing.T) {
	p := &provider.MockFailing{SimulateTimeout: true}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.FetchDebts(ctx, "ABC1234")
	if err == nil {
		t.Fatal("expected context deadline exceeded")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
}

// assertDebts verifies both providers return the same normalised data.
func assertDebts(t *testing.T, name string, debts []entity.Debt) {
	t.Helper()

	if len(debts) != 2 {
		t.Fatalf("%s: expected 2 debts, got %d", name, len(debts))
	}

	ipva := findByType(debts, entity.DebtTypeIPVA)
	if ipva == nil {
		t.Fatalf("%s: no IPVA debt found", name)
	}
	if ipva.Amount != 1500.00 {
		t.Errorf("%s: IPVA amount = %.2f, want 1500.00", name, ipva.Amount)
	}
	if ipva.DueDate != time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC) {
		t.Errorf("%s: IPVA due_date = %v, want 2024-01-10", name, ipva.DueDate)
	}

	multa := findByType(debts, entity.DebtTypeMULTA)
	if multa == nil {
		t.Fatalf("%s: no MULTA debt found", name)
	}
	if multa.Amount != 300.50 {
		t.Errorf("%s: MULTA amount = %.2f, want 300.50", name, multa.Amount)
	}
}

func findByType(debts []entity.Debt, t entity.DebtType) *entity.Debt {
	for i := range debts {
		if debts[i].Type == t {
			return &debts[i]
		}
	}
	return nil
}
