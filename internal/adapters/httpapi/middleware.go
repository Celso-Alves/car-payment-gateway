package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover wraps an http.Handler to log panics and return HTTP 500.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"erro interno"}` + "\n"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
