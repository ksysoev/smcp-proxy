package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// RequestLogger is middleware that logs HTTP requests
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response wrapper to capture the status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default status code
			}

			// Log the request
			logger.Debug("Request started",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Log the response
			logger.Debug("Request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"duration", time.Since(start),
			)
		})
	}
}

// responseWriter is a wrapper around http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the implicit 200 status code when Write is called without WriteHeader
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}