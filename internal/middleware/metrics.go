package middleware

import (
	"net/http"
	"time"
)

// Metrics creates a middleware that collects metrics for HTTP requests
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create response wrapper to capture status code
			ww := &metricsResponseWriter{w: w, status: http.StatusOK}

			// Process request
			next.ServeHTTP(ww, r)

			// Record metrics
			duration := time.Since(start)

			// In a real implementation, we would send metrics to a metrics system
			// For now, we just define the metrics we would track
			_ = "request_count"
			_ = "request_duration_seconds"
			_ = "request_size_bytes"
			_ = "response_size_bytes"
			_ = "http_status_code"
			
			// Labels we would use:
			_ = r.Method
			_ = r.URL.Path
			_ = ww.status
			_ = duration
		})
	}
}

// metricsResponseWriter is a wrapper around http.ResponseWriter that captures metrics
type metricsResponseWriter struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (rw *metricsResponseWriter) Header() http.Header {
	return rw.w.Header()
}

func (rw *metricsResponseWriter) Write(b []byte) (int, error) {
	size, err := rw.w.Write(b)
	rw.size += size
	return size, err
}

func (rw *metricsResponseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.w.WriteHeader(statusCode)
}
