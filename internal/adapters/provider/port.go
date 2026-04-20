// Package provider defines the port interfaces that all vehicle debt
// providers must satisfy (SPEC-001). Adapters live alongside this file;
// the domain and application layers depend only on this interface, never
// on any concrete adapter.
package provider

import (
	"context"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
)

// Provider is the port (SPEC-001) that every external debt source must
// implement. The Name method is used for structured logging and tracing.
//
// Implementations:
//   - ProviderA  — JSON response format (simulates SEFAZ-SP style API)
//   - ProviderB  — XML response format (simulates legacy DETRAN-SP style)
//   - MockFailing — always fails; used to demo fallback behavior
type Provider interface {
	// Name returns a human-readable identifier used in logs.
	Name() string

	// FetchDebts queries the provider for all outstanding debts associated
	// with the given vehicle plate. The context carries the per-provider
	// timeout (see SPEC-005). Returns a non-nil error on any failure,
	// including timeouts, so the caller can fall back to the next provider.
	FetchDebts(ctx context.Context, plate string) ([]entity.Debt, error)
}
