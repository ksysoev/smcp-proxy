package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"testing"

	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/stretchr/testify/assert"
)

func TestRecovery(t *testing.T) {
	// Create a logger that writes to a buffer so we can inspect the logs
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create a handler that will panic
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/panic":
			panic("test panic")
		case "/panic-error":
			panic(assert.AnError)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		}
	})

	// Create the middleware
	middleware := Recovery(logger)
	wrappedHandler := middleware(panicHandler)

	t.Run("Recovers from panic with string", func(t *testing.T) {
		// Reset the log buffer
		logBuf.Reset()

		// Make a request that will panic
		req := test.NewTestRequest("GET", "/panic", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		w := test.NewTestResponse()

		// Call the handler
		wrappedHandler.ServeHTTP(w, req)

		// Verify the response is an internal server error
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "Internal Server Error\n", w.Body.String())

		// Verify log contains the panic information
		logStr := logBuf.String()
		assert.Contains(t, logStr, "HTTP handler panic recovered")
		assert.Contains(t, logStr, "error=\"test panic\"")
		assert.Contains(t, logStr, "path=/panic")
		assert.Contains(t, logStr, "method=GET")
		assert.Contains(t, logStr, "remote_addr=127.0.0.1:12345")
		assert.Contains(t, logStr, "stack=")
	})

	t.Run("Recovers from panic with error", func(t *testing.T) {
		// Reset the log buffer
		logBuf.Reset()

		// Make a request that will panic with an error
		req := test.NewTestRequest("POST", "/panic-error", nil)
		w := test.NewTestResponse()

		// Call the handler
		wrappedHandler.ServeHTTP(w, req)

		// Verify the response is an internal server error
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// Verify log contains the panic information
		logStr := logBuf.String()
		assert.Contains(t, logStr, "HTTP handler panic recovered")
		assert.Contains(t, logStr, "error=\"assert.AnError general error for testing\"")
		assert.Contains(t, logStr, "path=/panic-error")
		assert.Contains(t, logStr, "method=POST")
	})

	t.Run("Passes through normal requests", func(t *testing.T) {
		// Reset the log buffer
		logBuf.Reset()

		// Make a normal request
		req := test.NewTestRequest("GET", "/normal", nil)
		w := test.NewTestResponse()

		// Call the handler
		wrappedHandler.ServeHTTP(w, req)

		// Verify the response is normal
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())

		// Verify no error was logged
		logStr := logBuf.String()
		assert.Empty(t, logStr)
	})
}
