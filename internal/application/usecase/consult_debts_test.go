package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/adapters/provider"
	"github.com/celsoadsjr/car-payment-gateway/internal/application/usecase"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
	"github.com/celsoadsjr/car-payment-gateway/pkg/logger"
	"github.com/shopspring/decimal"
)

var referenceDate = time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)

func buildUseCase(providers ...provider.Provider) *usecase.ConsultDebts {
	return buildUseCaseWithRetry(1, 0, providers...)
}

func buildUseCaseWithRetry(maxAttempts int, retryBackoff time.Duration, providers ...provider.Provider) *usecase.ConsultDebts {
	log := logger.NewDiscard()
	calc := service.NewCalculator(referenceDate, log)
	sim := service.NewSimulator()
	return usecase.NewConsultDebts(providers, calc, sim, log, 3*time.Second, 30*time.Second, maxAttempts, retryBackoff)
}

// flakyProvider fails the first failuresBeforeSuccess calls to FetchDebts, then delegates to ok (or ProviderA).
type flakyProvider struct {
	failuresBeforeSuccess int
	calls                 int
	ok                    provider.Provider
}

func (f *flakyProvider) Name() string { return "FlakyProvider" }

func (f *flakyProvider) FetchDebts(ctx context.Context, plate string) ([]entity.Debt, error) {
	f.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.calls <= f.failuresBeforeSuccess {
		return nil, errors.New("transient error")
	}
	if f.ok != nil {
		return f.ok.FetchDebts(ctx, plate)
	}
	return provider.NewProviderA().FetchDebts(ctx, plate)
}

// unknownTypeProvider returns only debts with an unregistered DebtType.
type unknownTypeProvider struct{}

func (unknownTypeProvider) Name() string { return "UnknownTypeProvider" }

func (unknownTypeProvider) FetchDebts(ctx context.Context, _ string) ([]entity.Debt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []entity.Debt{{
		Type:    entity.DebtType("LICENCIAMENTO"),
		Amount:  decimal.RequireFromString("100.00"),
		DueDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}}, nil
}

func TestConsultDebts_Execute_ProviderASuccess(t *testing.T) {
	uc := buildUseCase(provider.NewProviderA())

	result, err := uc.Execute(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Plate != "ABC1234" {
		t.Errorf("plate = %q, want ABC1234", result.Plate)
	}
	if len(result.Payment.Options) == 0 {
		t.Error("expected payment options, got none")
	}
}

func TestConsultDebts_Execute_ProviderBSuccess(t *testing.T) {
	uc := buildUseCase(provider.NewProviderB())

	result, err := uc.Execute(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := decimal.RequireFromString("1800.50")
	if !result.Summary.TotalOriginal.Equal(want) {
		t.Errorf("TotalOriginal = %s, want %s", result.Summary.TotalOriginal, want)
	}
}

func TestConsultDebts_Execute_FallbackToProviderB(t *testing.T) {
	uc := buildUseCase(
		&provider.MockFailing{},
		provider.NewProviderA(),
	)

	result, err := uc.Execute(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if result.Plate != "ABC1234" {
		t.Errorf("plate = %q, want ABC1234", result.Plate)
	}
}

func TestConsultDebts_Execute_AllProvidersFail(t *testing.T) {
	uc := buildUseCase(
		&provider.MockFailing{},
		&provider.MockFailing{},
	)

	_, err := uc.Execute(context.Background(), "ABC1234")
	if err == nil {
		t.Fatal("expected error when all providers fail, got nil")
	}
	if !errors.Is(err, usecase.ErrAllProvidersFailed) {
		t.Errorf("error = %v, want ErrAllProvidersFailed", err)
	}
}

func TestConsultDebts_Execute_ProviderTimeout(t *testing.T) {
	log := logger.NewDiscard()
	uc := usecase.NewConsultDebts(
		[]provider.Provider{
			&provider.MockFailing{SimulateTimeout: true},
			provider.NewProviderA(),
		},
		service.NewCalculator(referenceDate, log),
		service.NewSimulator(),
		log,
		100*time.Millisecond,
		30*time.Second,
		1,
		0,
	)

	result, err := uc.Execute(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("expected fallback after timeout, got error: %v", err)
	}
	if result.Plate != "ABC1234" {
		t.Errorf("plate = %q, want ABC1234", result.Plate)
	}
}

func TestConsultDebts_Execute_RetryThenSuccess(t *testing.T) {
	p := &flakyProvider{failuresBeforeSuccess: 2}
	uc := buildUseCaseWithRetry(3, 0, p)

	result, err := uc.Execute(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if p.calls != 3 {
		t.Fatalf("calls = %d, want 3", p.calls)
	}
	if result.Plate != "ABC1234" {
		t.Errorf("plate = %q, want ABC1234", result.Plate)
	}
}

func TestConsultDebts_Execute_RetryExhaustedThenFallback(t *testing.T) {
	flaky := &flakyProvider{failuresBeforeSuccess: 3}
	uc := buildUseCaseWithRetry(3, 0, flaky, provider.NewProviderA())

	result, err := uc.Execute(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("expected fallback success: %v", err)
	}
	if flaky.calls != 3 {
		t.Fatalf("flaky calls = %d, want 3", flaky.calls)
	}
	if result.Plate != "ABC1234" {
		t.Errorf("plate = %q, want ABC1234", result.Plate)
	}
}

func TestConsultDebts_Execute_BackoffRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	p := &flakyProvider{failuresBeforeSuccess: 10}
	uc := buildUseCaseWithRetry(2, 200*time.Millisecond, p)

	start := time.Now()
	_, err := uc.Execute(ctx, "ABC1234")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when context cancelled during backoff")
	}
	if !errors.Is(err, usecase.ErrAllProvidersFailed) {
		t.Fatalf("error = %v, want ErrAllProvidersFailed", err)
	}
	if elapsed > 80*time.Millisecond {
		t.Fatalf("expected early cancel, elapsed=%v", elapsed)
	}
}

func TestConsultDebts_Execute_PaymentOptions(t *testing.T) {
	uc := buildUseCase(provider.NewProviderA())

	result, err := uc.Execute(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	optionTypes := make(map[string]bool)
	for _, o := range result.Payment.Options {
		optionTypes[o.Type] = true
	}

	required := []string{"TOTAL", "SOMENTE_IPVA", "SOMENTE_MULTA"}
	for _, r := range required {
		if !optionTypes[r] {
			t.Errorf("missing payment option %q", r)
		}
	}
}

func TestConsultDebts_Execute_CancelledContext(t *testing.T) {
	uc := buildUseCase(provider.NewProviderA())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := uc.Execute(ctx, "ABC1234")
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
	if !errors.Is(err, usecase.ErrAllProvidersFailed) {
		t.Fatalf("error = %v, want wrapped ErrAllProvidersFailed", err)
	}
}

func TestConsultDebts_Execute_AllUnknownDebtTypes(t *testing.T) {
	uc := buildUseCase(unknownTypeProvider{})

	_, err := uc.Execute(context.Background(), "ABC1234")
	if err == nil {
		t.Fatal("expected error when all debts have unknown types")
	}
	if !errors.Is(err, usecase.ErrAllDebtsUnknownType) {
		t.Fatalf("error = %v, want ErrAllDebtsUnknownType", err)
	}
}
