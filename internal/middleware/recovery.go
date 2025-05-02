package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery is middleware that recovers from panics
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the error and stack trace
					logger.Error("HTTP handler panic recovered",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"remote_addr", r.RemoteAddr,
						"stack", string(debug.Stack()),
					)

					// Return an internal server error
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}
