package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/adapters/provider"
	"github.com/celsoadsjr/car-payment-gateway/internal/application/usecase"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
	"github.com/celsoadsjr/car-payment-gateway/pkg/logger"
)

var referenceDate = time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)

func buildUseCase(providers ...provider.Provider) *usecase.ConsultDebts {
	calc := service.NewCalculator(referenceDate)
	sim := service.NewSimulator()
	log := logger.New()
	return usecase.NewConsultDebts(providers, calc, sim, log, 3*time.Second)
}

func TestConsultDebts_Execute_ProviderASuccess(t *testing.T) {
	uc := buildUseCase(provider.NewProviderA("ABC1234"))

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
	uc := buildUseCase(provider.NewProviderB("ABC1234"))

	result, err := uc.Execute(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary.TotalOriginal != 1800.50 {
		t.Errorf("TotalOriginal = %.2f, want 1800.50", result.Summary.TotalOriginal)
	}
}

func TestConsultDebts_Execute_FallbackToProviderB(t *testing.T) {
	// MockFailing is first — use case must skip it and succeed with ProviderA.
	uc := buildUseCase(
		&provider.MockFailing{},
		provider.NewProviderA("ABC1234"),
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
	// SimulateTimeout=true causes MockFailing to block until ctx is cancelled.
	uc := usecase.NewConsultDebts(
		[]provider.Provider{
			&provider.MockFailing{SimulateTimeout: true},
			provider.NewProviderA("ABC1234"),
		},
		service.NewCalculator(referenceDate),
		service.NewSimulator(),
		logger.New(),
		100*time.Millisecond, // very short timeout to keep test fast
	)

	result, err := uc.Execute(context.Background(), "ABC1234")
	if err != nil {
		t.Fatalf("expected fallback after timeout, got error: %v", err)
	}
	if result.Plate != "ABC1234" {
		t.Errorf("plate = %q, want ABC1234", result.Plate)
	}
}

func TestConsultDebts_Execute_PaymentOptions(t *testing.T) {
	uc := buildUseCase(provider.NewProviderA("ABC1234"))

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
