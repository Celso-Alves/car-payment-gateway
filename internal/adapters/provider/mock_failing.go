package provider

import (
	"context"
	"errors"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
)

// ErrProviderUnavailable is returned by MockFailing to simulate a real
// provider outage (connection refused, 503, etc.).
var ErrProviderUnavailable = errors.New("provider unavailable")

// MockFailing is a test/demo provider that always fails (SPEC-005).
// Its purpose is to verify the fallback mechanism in ConsultDebts:
// placing it first in the provider list proves that the use case skips it
// and falls through to the next healthy provider.
//
// It can also simulate a slow provider by sleeping before returning,
// which exercises the context timeout path.
type MockFailing struct {
	// SimulateTimeout causes the mock to sleep until the context is done
	// (or until a long timer fires if the context has no deadline).
	SimulateTimeout bool
}

func (m *MockFailing) Name() string { return "MockFailing" }

func (m *MockFailing) FetchDebts(ctx context.Context, _ string) ([]entity.Debt, error) {
	if m.SimulateTimeout {
		t := time.NewTimer(30 * time.Second)
		defer t.Stop()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.C:
			return nil, errors.New("mock: unexpected timeout branch without context cancellation")
		}
	}
	return nil, ErrProviderUnavailable
}
