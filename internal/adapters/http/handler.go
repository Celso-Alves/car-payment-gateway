// Package http contains the HTTP adapter layer.
// It translates HTTP concerns (request parsing, status codes, JSON encoding)
// into use-case calls and back. No business logic lives here.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/celsoadsjr/car-payment-gateway/internal/application/usecase"
	"github.com/celsoadsjr/car-payment-gateway/internal/domain/entity"
)

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
	consultUC *usecase.ConsultDebts
	logger    *slog.Logger
}

// NewHandler constructs an HTTP Handler.
func NewHandler(uc *usecase.ConsultDebts, logger *slog.Logger) *Handler {
	return &Handler{consultUC: uc, logger: logger}
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
	var req consultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	plate := strings.TrimSpace(strings.ToUpper(req.Plate))
	if plate == "" {
		h.writeError(w, http.StatusBadRequest, "campo 'placa' é obrigatório")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	start := time.Now()
	result, err := h.consultUC.Execute(ctx, plate)
	elapsed := time.Since(start)

	if err != nil {
		h.logger.Error("consult failed",
			slog.String("plate", plate),
			slog.Duration("latency", elapsed),
			slog.String("error", err.Error()),
		)
		if errors.Is(err, usecase.ErrAllProvidersFailed) {
			h.writeError(w, http.StatusServiceUnavailable, "todos os provedores estão indisponíveis")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "erro interno")
		return
	}

	h.logger.Info("consult succeeded",
		slog.String("plate", plate),
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

// ConsultResult re-exported for handler test convenience.
type ConsultResult = entity.ConsultResult
