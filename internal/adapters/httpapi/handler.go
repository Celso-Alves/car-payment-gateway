// Package httpapi contains the HTTP adapter layer.
// It translates HTTP concerns (request parsing, status codes, JSON encoding)
// into use-case calls and back. No business logic lives here.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/application/usecase"
	"github.com/celsoadsjr/car-payment-gateway/pkg/logger"
)

const maxRequestBodyBytes int64 = 1 << 20 // 1 MiB

var platePattern = regexp.MustCompile(`^[A-Z]{3}-?[0-9][A-Z0-9][0-9]{2}$`)

// consultRequest is the JSON body expected by POST /api/v1/debts (SPEC-006).
type consultRequest struct {
	Plate string `json:"placa"`
}

// errorResponse is the standard error envelope returned on failures.
type errorResponse struct {
	Error string `json:"error"`
}

// Handler holds the dependencies needed to serve HTTP requests.
type Handler struct {
	consultUC       *usecase.ConsultDebts
	logger          *slog.Logger
	requestTimeout  time.Duration
}

// NewHandler constructs an HTTP Handler.
func NewHandler(uc *usecase.ConsultDebts, log *slog.Logger, requestTimeout time.Duration) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{consultUC: uc, logger: log, requestTimeout: requestTimeout}
}

// RegisterRoutes wires all routes into the provided ServeMux.
// Using the standard library mux avoids external router dependencies
// while still supporting method-scoped routing (Go 1.22+).
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/debts", h.consultDebts)
	mux.HandleFunc("GET /health", h.health)
}

// consultDebts handles POST /api/v1/debts (SPEC-006).
func (h *Handler) consultDebts(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req consultRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		if isRequestBodyTooLarge(err) {
			h.writeError(w, http.StatusRequestEntityTooLarge, "corpo da requisição excede o limite permitido")
			return
		}
		h.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	plate := strings.TrimSpace(strings.ToUpper(req.Plate))
	if plate == "" {
		h.writeError(w, http.StatusBadRequest, "campo 'placa' é obrigatório")
		return
	}
	if !platePattern.MatchString(plate) {
		h.writeError(w, http.StatusBadRequest, "formato de placa inválido")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.requestTimeout)
	defer cancel()

	start := time.Now()
	result, err := h.consultUC.Execute(ctx, plate)
	elapsed := time.Since(start)
	masked := logger.MaskPlate(plate)

	if err != nil {
		h.logger.Error("consult failed",
			slog.String("plate", masked),
			slog.Duration("latency", elapsed),
			slog.String("error", err.Error()),
		)
		if errors.Is(err, usecase.ErrAllProvidersFailed) {
			h.writeError(w, http.StatusServiceUnavailable, "todos os provedores estão indisponíveis")
			return
		}
		if errors.Is(err, usecase.ErrAllDebtsUnknownType) {
			h.writeError(w, http.StatusBadRequest, "todos os débitos possuem tipo desconhecido")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "erro interno")
		return
	}

	h.logger.Info("consult succeeded",
		slog.String("plate", masked),
		slog.Duration("latency", elapsed),
	)
	h.writeJSON(w, http.StatusOK, result)
}

// health is a simple liveness probe endpoint.
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Error("failed to encode response", slog.String("error", err.Error()))
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, errorResponse{Error: msg})
}

// isRequestBodyTooLarge matches errors from http.MaxBytesReader (the stdlib
// returns a string error and does not export http.ErrBodyTooLarge in all Go versions).
func isRequestBodyTooLarge(err error) bool {
	const msg = "http: request body too large"
	for e := err; e != nil; e = errors.Unwrap(e) {
		if strings.Contains(e.Error(), msg) {
			return true
		}
	}
	return false
}
