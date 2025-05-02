package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"testing"

	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/stretchr/testify/assert"
)

func TestRequestLogger(t *testing.T) {
	// Create a logger that writes to a buffer so we can inspect the logs
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create a test handler that will be wrapped by the middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test writing different status codes
		switch r.URL.Path {
		case "/success":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
		case "/no-write-header":
			// Test implicit status code
			_, _ = w.Write([]byte("no explicit status code"))
		}
	})

	// Create the middleware
	middleware := RequestLogger(logger)
	wrappedHandler := middleware(testHandler)

	t.Run("Logs successful request", func(t *testing.T) {
		// Reset the log buffer
		logBuf.Reset()

		// Make a successful request
		req := test.NewTestRequest("GET", "/success", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("User-Agent", "test-agent")
		w := test.NewTestResponse()

		// Call the handler
		wrappedHandler.ServeHTTP(w, req)

		// Verify the response
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())

		// Verify log contains start and completed messages
		logStr := logBuf.String()
		assert.Contains(t, logStr, "Request started")
		assert.Contains(t, logStr, "method=GET")
		assert.Contains(t, logStr, "path=/success")
		assert.Contains(t, logStr, "remote_addr=127.0.0.1:12345")
		assert.Contains(t, logStr, "user_agent=test-agent")

		assert.Contains(t, logStr, "Request completed")
		assert.Contains(t, logStr, "status=200")
		assert.Contains(t, logStr, "duration=")
	})

	t.Run("Logs error request", func(t *testing.T) {
		// Reset the log buffer
		logBuf.Reset()

		// Make an error request
		req := test.NewTestRequest("POST", "/error", nil)
		w := test.NewTestResponse()

		// Call the handler
		wrappedHandler.ServeHTTP(w, req)

		// Verify the response
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "error", w.Body.String())

		// Verify log contains the error status
		logStr := logBuf.String()
		assert.Contains(t, logStr, "Request completed")
		assert.Contains(t, logStr, "method=POST")
		assert.Contains(t, logStr, "path=/error")
		assert.Contains(t, logStr, "status=500")
	})

	t.Run("Handles request with no explicit status code", func(t *testing.T) {
		// Reset the log buffer
		logBuf.Reset()

		// Make a request that doesn't call WriteHeader
		req := test.NewTestRequest("GET", "/no-write-header", nil)
		w := test.NewTestResponse()

		// Call the handler
		wrappedHandler.ServeHTTP(w, req)

		// Verify the default status is 200
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify log contains the default status
		logStr := logBuf.String()
		assert.Contains(t, logStr, "Request completed")
		assert.Contains(t, logStr, "path=/no-write-header")
		assert.Contains(t, logStr, "status=200")
	})
}
