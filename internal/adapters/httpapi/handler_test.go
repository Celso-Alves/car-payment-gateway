package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/adapters/httpapi"
	"github.com/celsoadsjr/car-payment-gateway/internal/adapters/provider"
	"github.com/celsoadsjr/car-payment-gateway/internal/application/usecase"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/service"
	"github.com/celsoadsjr/car-payment-gateway/pkg/logger"
)

var referenceDate = time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)

func newTestHandler(providers ...provider.Provider) *httpapi.Handler {
	log := logger.NewDiscard()
	calc := service.NewCalculator(referenceDate, log)
	sim := service.NewSimulator()
	uc := usecase.NewConsultDebts(providers, calc, sim, log, 3*time.Second, 30*time.Second, 1, 0)
	return httpapi.NewHandler(uc, log, 30*time.Second)
}

func TestHandler_ConsultDebts_Success(t *testing.T) {
	h := newTestHandler(provider.NewProviderA())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"placa":"ABC1234"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debts", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var result entity.ConsultResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Plate != "ABC1234" {
		t.Errorf("placa = %q, want ABC1234", result.Plate)
	}
	if len(result.Payment.Options) == 0 {
		t.Error("expected payment options")
	}
}

func TestHandler_ConsultDebts_MissingPlate(t *testing.T) {
	h := newTestHandler(provider.NewProviderA())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"placa":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debts", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandler_ConsultDebts_InvalidPlate(t *testing.T) {
	h := newTestHandler(provider.NewProviderA())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"placa":"INVALID"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debts", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandler_ConsultDebts_InvalidJSON(t *testing.T) {
	h := newTestHandler(provider.NewProviderA())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bytes.NewBufferString(`not-json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debts", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandler_ConsultDebts_BodyTooLarge(t *testing.T) {
	h := newTestHandler(provider.NewProviderA())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Valid JSON with a huge string value so the decoder must read past 1 MiB.
	bodyStr := `{"placa":"` + strings.Repeat("A", (1<<20)+500) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debts", strings.NewReader(bodyStr))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ConsultDebts_AllProvidersFail(t *testing.T) {
	h := newTestHandler(&provider.MockFailing{}, &provider.MockFailing{})

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"placa":"ABC1234"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debts", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestHandler_Health(t *testing.T) {
	h := newTestHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
