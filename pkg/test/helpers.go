package test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// NewTestLogger creates a new logger for testing
func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// NewLoggerWithBuffer creates a logger that writes to a buffer
func NewLoggerWithBuffer() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	return logger, buf
}

// NewTestRequest creates a new test HTTP request
func NewTestRequest(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// NewTestResponse creates a new test HTTP response recorder
func NewTestResponse() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

// SetEnv sets an environment variable for the duration of the test
func SetEnv(t *testing.T, key, value string) {
	originalValue, exists := os.LookupEnv(key)

	err := os.Setenv(key, value)
	if err != nil {
		t.Fatalf("Failed to set environment variable %s: %v", key, err)
	}

	t.Cleanup(func() {
		if exists {
			if err := os.Setenv(key, originalValue); err != nil {
				t.Logf("Failed to restore environment variable %s: %v", key, err)
			}
		} else {
			if err := os.Unsetenv(key); err != nil {
				t.Logf("Failed to unset environment variable %s: %v", key, err)
			}
		}
	})
}

// TempFile creates a temporary file with the given content and returns its path
func TempFile(t *testing.T, content string) string {
	file, err := os.CreateTemp("", "smcp-proxy-test-*.tmp")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	path := file.Name()

	_, err = file.WriteString(content)
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	err = file.Close()
	if err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Remove(path); err != nil {
			t.Logf("Failed to remove temporary file %s: %v", path, err)
		}
	})

	return path
}
