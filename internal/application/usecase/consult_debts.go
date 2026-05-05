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
//  2. Retries each provider up to maxAttemptsPerProvider times with optional backoff.
//  3. Applies interest calculation via the domain Calculator.
//  4. Generates payment options via the domain Simulator.
//
// It depends on the Provider port interface, never on concrete adapters,
// so new providers can be added by registering them in main.go.
type ConsultDebts struct {
	providers              []provider.Provider
	calculator             *service.Calculator
	simulator              *service.Simulator
	logger                 *slog.Logger
	providerTimeout        time.Duration
	requestTimeout         time.Duration
	maxAttemptsPerProvider int
	retryBackoff           time.Duration
}

// NewConsultDebts constructs the use case with all required dependencies.
// providerTimeout controls how long the use case waits for each individual
// FetchDebts call before treating it as failed (same provider may be retried).
// requestTimeout is the HTTP-layer deadline; if the estimated worst-case chain
// exceeds it, a warning is logged (providers may not all be tried in time).
// maxAttemptsPerProvider must be at least 1 (values below 1 are clamped to 1).
// retryBackoff is the pause between retries on the same provider; zero disables backoff.
func NewConsultDebts(
	providers []provider.Provider,
	calculator *service.Calculator,
	simulator *service.Simulator,
	logger *slog.Logger,
	providerTimeout time.Duration,
	requestTimeout time.Duration,
	maxAttemptsPerProvider int,
	retryBackoff time.Duration,
) *ConsultDebts {
	if logger == nil {
		logger = slog.Default()
	}
	if maxAttemptsPerProvider < 1 {
		maxAttemptsPerProvider = 1
	}

	perProviderWorst := time.Duration(maxAttemptsPerProvider)*providerTimeout +
		time.Duration(max(0, maxAttemptsPerProvider-1))*retryBackoff
	maxChain := time.Duration(len(providers)) * perProviderWorst
	if requestTimeout > 0 && maxChain > requestTimeout {
		logger.Warn("provider chain may exceed HTTP request timeout",
			slog.Int("providers", len(providers)),
			slog.Int("maxAttemptsPerProvider", maxAttemptsPerProvider),
			slog.Duration("providerTimeout", providerTimeout),
			slog.Duration("retryBackoff", retryBackoff),
			slog.Duration("maxChain", maxChain),
			slog.Duration("requestTimeout", requestTimeout),
		)
	}
	return &ConsultDebts{
		providers:              providers,
		calculator:             calculator,
		simulator:              simulator,
		logger:                 logger,
		providerTimeout:        providerTimeout,
		requestTimeout:         requestTimeout,
		maxAttemptsPerProvider: maxAttemptsPerProvider,
		retryBackoff:           retryBackoff,
	}
}

// Execute runs the full debt consultation and payment simulation for a plate.
//
// Fallback strategy (SPEC-005): providers are tried in order. Each provider
// may be retried up to maxAttemptsPerProvider times. On any terminal failure
// for that provider, the use case tries the next one. If all providers fail,
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
// successful response. Each FetchDebts call gets its own timeout-bounded
// context so a slow provider does not exhaust the caller's overall deadline.
func (uc *ConsultDebts) fetchWithFallback(ctx context.Context, plate string) ([]entity.Debt, error) {
	var lastErr error
	masked := logger.MaskPlate(plate)

	for _, p := range uc.providers {
		if err := ctx.Err(); err != nil {
			return nil, terminalAllFailed(lastErr, err)
		}

		var lastAttemptErr error
		for attempt := 1; attempt <= uc.maxAttemptsPerProvider; attempt++ {
			if err := ctx.Err(); err != nil {
				return nil, terminalAllFailed(lastErr, err)
			}

			pCtx, cancel := context.WithTimeout(ctx, uc.providerTimeout)
			start := time.Now()
			debts, err := p.FetchDebts(pCtx, plate)
			elapsed := time.Since(start)
			cancel()

			if err == nil {
				uc.logger.Info("provider succeeded",
					slog.String("provider", p.Name()),
					slog.Int("attempt", attempt),
					slog.Int("maxAttempts", uc.maxAttemptsPerProvider),
					slog.Duration("latency", elapsed),
					slog.String("plate", masked),
				)
				return debts, nil
			}
			lastAttemptErr = err

			if ctx.Err() != nil {
				return nil, fmt.Errorf("%w: %w", ErrAllProvidersFailed, ctx.Err())
			}

			if attempt < uc.maxAttemptsPerProvider {
				uc.logger.Warn("provider attempt failed, retrying",
					slog.String("provider", p.Name()),
					slog.Int("attempt", attempt),
					slog.Int("maxAttempts", uc.maxAttemptsPerProvider),
					slog.Duration("latency", elapsed),
					slog.String("plate", masked),
					slog.String("error", err.Error()),
				)
				if uc.retryBackoff > 0 {
					select {
					case <-time.After(uc.retryBackoff):
					case <-ctx.Done():
						return nil, fmt.Errorf("%w: %w", ErrAllProvidersFailed, ctx.Err())
					}
				}
				continue
			}

			uc.logger.Warn("provider failed, trying next",
				slog.String("provider", p.Name()),
				slog.Int("attempt", attempt),
				slog.Int("maxAttempts", uc.maxAttemptsPerProvider),
				slog.Duration("latency", elapsed),
				slog.String("plate", masked),
				slog.String("error", err.Error()),
			)
			lastErr = fmt.Errorf("provider %s: %w", p.Name(), lastAttemptErr)
		}
	}

	return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
}

func terminalAllFailed(lastErr, ctxErr error) error {
	if lastErr != nil {
		return fmt.Errorf("%w: last error: %v; context done: %v", ErrAllProvidersFailed, lastErr, ctxErr)
	}
	return fmt.Errorf("%w: %w", ErrAllProvidersFailed, ctxErr)
}
