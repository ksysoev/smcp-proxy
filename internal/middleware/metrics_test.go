package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsMiddleware(t *testing.T) {
	// Create a test handler that we'll wrap with the metrics middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	// Create the metrics middleware
	middleware := Metrics()

	// Wrap the test handler with the middleware
	handler := middleware(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)

	// Create a response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(rec, req)

	// Check that the response was processed correctly
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test response", rec.Body.String())
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
}

func TestMetricsResponseWriter(t *testing.T) {
	// Create a standard response recorder
	rec := httptest.NewRecorder()

	// Create our metrics response writer with initial status of OK
	mrw := &metricsResponseWriter{w: rec, status: http.StatusOK}

	// Test Header method
	mrw.Header().Set("X-Test", "test value")
	assert.Equal(t, "test value", rec.Header().Get("X-Test"))

	// Test Write method
	n, err := mrw.Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, 4, mrw.size)
	assert.Equal(t, "test", rec.Body.String())

	// Test WriteHeader method
	mrw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, mrw.status)

	// Test multiple writes
	n, err = mrw.Write([]byte(" more"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, 9, mrw.size)
	assert.Equal(t, "test more", rec.Body.String())
}

func TestMetricsWithErrorResponse(t *testing.T) {
	// Create a test handler that returns an error status
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	})

	// Create the metrics middleware
	middleware := Metrics()

	// Wrap the error handler with the middleware
	handler := middleware(errorHandler)

	// Create a test request
	req := httptest.NewRequest("POST", "http://example.com/error", nil)

	// Create a response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(rec, req)

	// Check that the response was processed correctly
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "server error", rec.Body.String())
}

// Test for HTTP2 response writer compatibility by implementing the Pusher interface
func TestMetricsResponseWriterWithPusher(t *testing.T) {
	// Create a mock ResponseWriter that implements http.Pusher
	mockRW := &mockPusherResponseWriter{
		httptest.NewRecorder(),
		false,
	}

	// Create our metrics response writer
	mrw := &metricsResponseWriter{w: mockRW, status: http.StatusOK}

	// Check if we can cast it to a Pusher
	pusher, ok := mrw.w.(http.Pusher)
	assert.True(t, ok, "Expected metrics response writer to expose Pusher interface")

	// Use the Pusher interface
	err := pusher.Push("/style.css", nil)
	assert.NoError(t, err)
	assert.True(t, mockRW.pushCalled, "Expected Push method to be called")
}

// Mock response writer that implements http.Pusher
type mockPusherResponseWriter struct {
	*httptest.ResponseRecorder
	pushCalled bool
}

func (m *mockPusherResponseWriter) Push(target string, opts *http.PushOptions) error {
	m.pushCalled = true
	return nil
}
