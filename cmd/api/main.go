// Command api is the entry point for the car-payment-gateway service.
// It wires all dependencies (providers, domain services, use cases, HTTP handler)
// using manual dependency injection — no DI framework required for a service
// of this size, and explicit wiring makes the dependency graph auditable.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpAdapter "github.com/celsoadsjr/car-payment-gateway/internal/adapters/http"
	"github.com/celsoadsjr/car-payment-gateway/internal/adapters/provider"
	"github.com/celsoadsjr/car-payment-gateway/internal/application/usecase"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
	"github.com/celsoadsjr/car-payment-gateway/pkg/logger"
)

// referenceDate is the fixed date specified in the test document.
// In a production system this would be time.Now() or injected via config.
var referenceDate = time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)

func main() {
	log := logger.New()

	addr := envOr("PORT", "8080")
	addr = ":" + addr

	// ── Provider chain (SPEC-001, SPEC-005) ────────────────────────────────
	// Providers are tried in order. Add/remove providers here to change the
	// fallback chain without touching any other file.
	//
	// Set ENABLE_MOCK_FAILING=true to prepend a failing provider and demo
	// the fallback mechanism during the live coding presentation.
	providers := buildProviders(log)

	// ── Domain services ─────────────────────────────────────────────────────
	calculator := service.NewCalculator(referenceDate)
	simulator := service.NewSimulator()

	// ── Use case ────────────────────────────────────────────────────────────
	uc := usecase.NewConsultDebts(
		providers,
		calculator,
		simulator,
		log,
		3*time.Second, // per-provider timeout
	)

	// ── HTTP adapter ─────────────────────────────────────────────────────────
	handler := httpAdapter.NewHandler(uc, log)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutting down gracefully")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", slog.String("error", err.Error()))
	}
}

// buildProviders constructs the ordered provider list.
// ENABLE_MOCK_FAILING=true prepends the failing mock to demonstrate fallback.
func buildProviders(log *slog.Logger) []provider.Provider {
	var providers []provider.Provider

	if os.Getenv("ENABLE_MOCK_FAILING") == "true" {
		log.Warn("MockFailing provider enabled — fallback demo mode active")
		providers = append(providers, &provider.MockFailing{})
	}

	providers = append(providers,
		provider.NewProviderA(""),
		provider.NewProviderB(""),
	)

	log.Info("providers registered", slog.Int("count", len(providers)))
	return providers
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
