// Command api is the entry point for the car-payment-gateway service.
// It wires all dependencies (providers, domain services, use cases, HTTP handler)
// using manual dependency injection — no DI framework required for a service
// of this size, and explicit wiring makes the dependency graph auditable.
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpapi "github.com/celsoadsjr/car-payment-gateway/internal/adapters/httpapi"
	"github.com/celsoadsjr/car-payment-gateway/internal/adapters/provider"
	"github.com/celsoadsjr/car-payment-gateway/internal/application/usecase"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
	"github.com/celsoadsjr/car-payment-gateway/pkg/logger"
)

func main() {
	log := logger.New()

	referenceDate := time.Now().UTC()
	if v := os.Getenv("REFERENCE_DATE"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			log.Error("invalid REFERENCE_DATE; expected YYYY-MM-DD", slog.String("value", v), slog.String("error", err.Error()))
			os.Exit(1)
		}
		referenceDate = parsed.UTC()
	}

	providerTimeout := 3 * time.Second
	requestTimeout := 10 * time.Second

	providers := buildProviders(log)

	calculator := service.NewCalculator(referenceDate, log)
	simulator := service.NewSimulator()

	uc := usecase.NewConsultDebts(
		providers,
		calculator,
		simulator,
		log,
		providerTimeout,
		requestTimeout,
	)

	handler := httpapi.NewHandler(uc, log, requestTimeout)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         listenAddr(),
		Handler:      httpapi.Recover(log)(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	case <-quit:
	}

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
		provider.NewProviderA(),
		provider.NewProviderB(),
	)

	log.Info("providers registered", slog.Int("count", len(providers)))
	return providers
}

// listenAddr returns ADDR if set, otherwise ":PORT" with PORT defaulting to 3000.
func listenAddr() string {
	if v := os.Getenv("ADDR"); v != "" {
		return v
	}
	port := envOr("PORT", "3000")
	return net.JoinHostPort("", port)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
