package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestIsConnectionRefused(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error returns false",
			err:  nil,
			want: false,
		},
		{
			name: "unrelated error returns false",
			err:  errors.New("something else went wrong"),
			want: false,
		},
		{
			name: "bare ECONNREFUSED returns true (fallback errors.Is branch)",
			err:  syscall.ECONNREFUSED,
			want: true,
		},
		{
			name: "wrapped ECONNREFUSED returns true (fallback errors.Is branch)",
			err:  fmt.Errorf("context: %w", syscall.ECONNREFUSED),
			want: true,
		},
		{
			name: "OpError dial+ECONNREFUSED returns true (primary branch)",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: syscall.ECONNREFUSED,
			},
			want: true,
		},
		{
			name: "OpError read+ECONNREFUSED returns true (primary branch)",
			err: &net.OpError{
				Op:  "read",
				Net: "tcp",
				Err: syscall.ECONNREFUSED,
			},
			want: true,
		},
		{
			name: "OpError dial+ETIMEDOUT returns false (primary branch, wrong errno)",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: syscall.ETIMEDOUT,
			},
			want: false,
		},
		{
			name: "OpError dial+plain error falls through to errors.Is and returns false (inner As miss)",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: errors.New("not a syscall errno"),
			},
			want: false,
		},
		{
			name: "OpError write+ECONNREFUSED returns true via fallback errors.Is",
			err: &net.OpError{
				Op:  "write",
				Net: "tcp",
				Err: syscall.ECONNREFUSED,
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsConnectionRefused(tc.err); got != tc.want {
				t.Errorf("IsConnectionRefused(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// fastRetryConfig keeps test runtime in the low-millisecond range.
func fastRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		BackoffFactor:  2.0,
	}
}

func TestRetryWithBackoff_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	err := RetryWithBackoff(context.Background(), fastRetryConfig(), func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("want 1 call, got %d", calls)
	}
}

func TestRetryWithBackoff_NonRetryableErrorReturnedImmediately(t *testing.T) {
	sentinel := errors.New("boom")
	calls := 0
	err := RetryWithBackoff(context.Background(), fastRetryConfig(), func() error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("want wrapped sentinel, got %v", err)
	}
	if calls != 1 {
		t.Errorf("want 1 call (no retry on non-conn-refused), got %d", calls)
	}
}

func TestRetryWithBackoff_SuccessAfterTransientRefusals(t *testing.T) {
	cfg := fastRetryConfig()
	cfg.MaxRetries = 5 // allow up to 6 attempts
	calls := 0
	err := RetryWithBackoff(context.Background(), cfg, func() error {
		calls++
		if calls < 3 {
			return syscall.ECONNREFUSED
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("want 3 calls (2 failures + 1 success), got %d", calls)
	}
}

func TestRetryWithBackoff_MaxRetriesExceededReturnsWrappedError(t *testing.T) {
	cfg := fastRetryConfig()
	cfg.MaxRetries = 2 // 3 total attempts (attempt 0, 1, 2)
	calls := 0
	err := RetryWithBackoff(context.Background(), cfg, func() error {
		calls++
		return syscall.ECONNREFUSED
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Errorf("expected wrapped ECONNREFUSED, got %v", err)
	}
	// The loop runs attempt = 0..MaxRetries inclusive.
	if calls != cfg.MaxRetries+1 {
		t.Errorf("want %d calls, got %d", cfg.MaxRetries+1, calls)
	}
	// Error message includes the retry count for operator diagnostics.
	if !containsAll(err.Error(), "max retries", "2") {
		t.Errorf("error message missing retry count: %q", err.Error())
	}
}

func TestRetryWithBackoff_ZeroMaxRetriesReturnsImmediately(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:     0,
		InitialBackoff: 1 * time.Second, // large on purpose; must not sleep
		MaxBackoff:     5 * time.Second,
		BackoffFactor:  2.0,
	}
	calls := 0
	start := time.Now()
	err := RetryWithBackoff(context.Background(), cfg, func() error {
		calls++
		return syscall.ECONNREFUSED
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Errorf("expected wrapped ECONNREFUSED, got %v", err)
	}
	if calls != 1 {
		t.Errorf("want 1 call (no retry budget), got %d", calls)
	}
	// The only sleep would be after the attempt, but attempt == MaxRetries is
	// short-circuited before the sleep select. So total runtime must be well
	// under InitialBackoff.
	if elapsed >= 500*time.Millisecond {
		t.Errorf("no-retry path should not have slept; elapsed %v", elapsed)
	}
}

func TestRetryWithBackoff_ContextCancelledDuringBackoff(t *testing.T) {
	// Use a slow backoff so cancellation is guaranteed to land during the sleep.
	cfg := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel shortly after the first failed attempt starts its backoff.
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	calls := 0
	err := RetryWithBackoff(ctx, cfg, func() error {
		calls++
		return syscall.ECONNREFUSED
	})
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected wrapped context.Canceled, got %v", err)
	}
	if calls < 1 {
		t.Errorf("expected at least 1 call before cancellation, got %d", calls)
	}
}

func TestRetryWithBackoff_WaitsBetweenAttempts(t *testing.T) {
	// Lower-bound timing check — with InitialBackoff=20ms and BackoffFactor=2,
	// three failed attempts sleep ~20ms + ~40ms = ~60ms before giving up.
	// Assert >= 50ms to absorb scheduler jitter.
	cfg := RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 20 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
	}
	start := time.Now()
	_ = RetryWithBackoff(context.Background(), cfg, func() error {
		return syscall.ECONNREFUSED
	})
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least ~60ms of backoff delay, got %v", elapsed)
	}
}

// containsAll reports whether s contains every substring in subs.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func TestDefaultRetryConfig(t *testing.T) {
	c := DefaultRetryConfig()
	if c.MaxRetries <= 0 {
		t.Errorf("MaxRetries = %d, want >0", c.MaxRetries)
	}
	if c.InitialBackoff <= 0 {
		t.Errorf("InitialBackoff = %v, want >0", c.InitialBackoff)
	}
	if c.MaxBackoff < c.InitialBackoff {
		t.Errorf("MaxBackoff (%v) should be >= InitialBackoff (%v)", c.MaxBackoff, c.InitialBackoff)
	}
	if c.BackoffFactor < 1.0 {
		t.Errorf("BackoffFactor = %v, want >=1.0", c.BackoffFactor)
	}
}
