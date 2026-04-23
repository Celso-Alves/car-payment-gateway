// Package usecase contains the application layer. It orchestrates domain
// services and adapters but contains no business rules of its own.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/adapters/provider"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
	"github.com/celsoadsjr/car-payment-gateway/pkg/logger"
)

// ErrAllProvidersFailed is returned when every provider in the list
// fails or times out (SPEC-005).
var ErrAllProvidersFailed = errors.New("all providers failed or are unavailable")

// ErrAllDebtsUnknownType is re-exported for callers that match errors without
// importing the domain service package.
var ErrAllDebtsUnknownType = service.ErrAllDebtsUnknownType

// ConsultDebts is the application Facade (SPEC-005) that:
//  1. Tries each provider in order, stopping at the first success (fallback).
//  2. Applies interest calculation via the domain Calculator.
//  3. Generates payment options via the domain Simulator.
//
// It depends on the Provider port interface, never on concrete adapters,
// so new providers can be added by registering them in main.go.
type ConsultDebts struct {
	providers       []provider.Provider
	calculator      *service.Calculator
	simulator       *service.Simulator
	logger          *slog.Logger
	providerTimeout time.Duration
	requestTimeout  time.Duration
}

// NewConsultDebts constructs the use case with all required dependencies.
// providerTimeout controls how long the use case waits for each individual
// provider before giving up and trying the next one.
// requestTimeout is the HTTP-layer deadline; if len(providers)*providerTimeout
// exceeds it, a warning is logged (providers may not all be tried in time).
func NewConsultDebts(
	providers []provider.Provider,
	calculator *service.Calculator,
	simulator *service.Simulator,
	logger *slog.Logger,
	providerTimeout time.Duration,
	requestTimeout time.Duration,
) *ConsultDebts {
	if logger == nil {
		logger = slog.Default()
	}
	maxChain := time.Duration(len(providers)) * providerTimeout
	if requestTimeout > 0 && maxChain > requestTimeout {
		logger.Warn("provider chain may exceed HTTP request timeout",
			slog.Int("providers", len(providers)),
			slog.Duration("providerTimeout", providerTimeout),
			slog.Duration("maxChain", maxChain),
			slog.Duration("requestTimeout", requestTimeout),
		)
	}
	return &ConsultDebts{
		providers:       providers,
		calculator:      calculator,
		simulator:       simulator,
		logger:          logger,
		providerTimeout: providerTimeout,
		requestTimeout:  requestTimeout,
	}
}

// Execute runs the full debt consultation and payment simulation for a plate.
//
// Fallback strategy (SPEC-005): providers are tried in order. On any error
// (network failure, timeout, bad response) the use case logs the failure,
// increments a counter, and tries the next provider. If all providers fail,
// ErrAllProvidersFailed is returned.
func (uc *ConsultDebts) Execute(ctx context.Context, plate string) (entity.ConsultResult, error) {
	debts, err := uc.fetchWithFallback(ctx, plate)
	if err != nil {
		return entity.ConsultResult{}, err
	}

	updatedDebts, err := uc.calculator.Apply(debts)
	if err != nil {
		return entity.ConsultResult{}, err
	}

	result := uc.simulator.Simulate(plate, updatedDebts)
	return result, nil
}

// fetchWithFallback iterates over all providers, returning the first
// successful response. Each attempt gets its own timeout-bounded context
// so a slow provider does not exhaust the caller's overall deadline.
func (uc *ConsultDebts) fetchWithFallback(ctx context.Context, plate string) ([]entity.Debt, error) {
	var lastErr error
	masked := logger.MaskPlate(plate)

	for _, p := range uc.providers {
		pCtx, cancel := context.WithTimeout(ctx, uc.providerTimeout)

		start := time.Now()
		debts, err := p.FetchDebts(pCtx, plate)
		elapsed := time.Since(start)
		cancel()

		if err == nil {
			uc.logger.Info("provider succeeded",
				slog.String("provider", p.Name()),
				slog.Duration("latency", elapsed),
				slog.String("plate", masked),
			)
			return debts, nil
		}

		uc.logger.Warn("provider failed, trying next",
			slog.String("provider", p.Name()),
			slog.Duration("latency", elapsed),
			slog.String("plate", masked),
			slog.String("error", err.Error()),
		)
		lastErr = fmt.Errorf("provider %s: %w", p.Name(), err)
	}

	return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
}
