package proxy

import "fmt"

// ErrInvalidBackendConfig represents an error for invalid backend configuration
type ErrInvalidBackendConfig string

func (e ErrInvalidBackendConfig) Error() string {
	return fmt.Sprintf("invalid backend configuration: %s", string(e))
}

// ErrBackendNotFound represents an error when a backend is not found
type ErrBackendNotFound string

func (e ErrBackendNotFound) Error() string {
	return fmt.Sprintf("backend not found: %s", string(e))
}

// ErrBackendStartFailed represents an error when a backend fails to start
type ErrBackendStartFailed struct {
	Cause error
	ID    string
}

func (e ErrBackendStartFailed) Error() string {
	return fmt.Sprintf("failed to start backend %s: %v", e.ID, e.Cause)
}

func (e ErrBackendStartFailed) Unwrap() error {
	return e.Cause
}
