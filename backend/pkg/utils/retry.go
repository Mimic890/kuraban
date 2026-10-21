package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		BackoffFactor:  2.0,
	}
}

// IsConnectionRefused checks if the error is a connection refused error
func IsConnectionRefused(err error) bool {
	if err == nil {
		return false
	}

	// Check for connection refused error
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// Check if it's a connection refused error
		if opErr.Op == "dial" || opErr.Op == "read" {
			var syscallErr *syscall.Errno
			if errors.As(opErr.Err, &syscallErr) {
				return *syscallErr == syscall.ECONNREFUSED
			}
		}
	}

	// Also check for "connection refused" in error message as fallback
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	// Some HTTP clients (e.g. azuretls) collapse the syscall error into a
	// plain string before returning. Fall back to substring matching so the
	// retry path still triggers for those wrappers. "connectex" is the
	// Windows variant emitted by the Go runtime.
	msg := err.Error()
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "actively refused") {
		return true
	}

	return false
}

// RetryWithBackoff executes a function with exponential backoff on connection refused errors
func RetryWithBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// If it's not a connection refused error, don't retry
		if !IsConnectionRefused(err) {
			return err
		}

		// If this was the last attempt, return the error
		if attempt == config.MaxRetries {
			return fmt.Errorf("max retries (%d) exceeded: %w", config.MaxRetries, err)
		}

		// Calculate backoff duration with exponential increase
		backoff := time.Duration(float64(config.InitialBackoff) * pow(config.BackoffFactor, float64(attempt)))
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return lastErr
}

// pow is a simple integer exponentiation helper
func pow(base float64, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}
