package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockLogger struct {
	debugCount int
	warnCount  int
	errorCount int
}

func (m *mockLogger) Debugf(format string, args ...interface{}) {
	m.debugCount++
}

func (m *mockLogger) Warnf(format string, args ...interface{}) {
	m.warnCount++
}

func (m *mockLogger) Errorf(format string, args ...interface{}) {
	m.errorCount++
}

func TestRetry_Success(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	config := &RetryConfig{
		MaxRetries:    5,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	logger := &mockLogger{}
	err := Retry(ctx, fn, config, logger)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}

	if logger.warnCount != 2 {
		t.Errorf("Expected 2 warnings, got %d", logger.warnCount)
	}
}

func TestRetry_MaxRetries(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("always fails")
	}

	config := &RetryConfig{
		MaxRetries:    2,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	logger := &mockLogger{}
	err := Retry(ctx, fn, config, logger)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if callCount != 3 { // initial + 2 retries
		t.Errorf("Expected 3 calls, got %d", callCount)
	}

	if logger.warnCount != 3 {
		t.Errorf("Expected 3 warnings, got %d", logger.warnCount)
	}
}

func TestRetry_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 2 {
			cancel()
		}
		return errors.New("always fails")
	}

	config := &RetryConfig{
		MaxRetries:    10,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	logger := &mockLogger{}
	err := Retry(ctx, fn, config, logger)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}

	if config.InitialDelay != 1*time.Second {
		t.Errorf("Expected InitialDelay 1s, got %v", config.InitialDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay 30s, got %v", config.MaxDelay)
	}

	if config.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor 2.0, got %f", config.BackoffFactor)
	}
}

func TestCalculateDelay(t *testing.T) {
	tests := []struct {
		name         string
		attempt      int
		initialDelay time.Duration
		backoff      float64
		maxDelay     time.Duration
		expected     time.Duration
	}{
		{
			name:         "first attempt",
			attempt:      0,
			initialDelay: 1 * time.Second,
			backoff:      2.0,
			maxDelay:     10 * time.Second,
			expected:     0,
		},
		{
			name:         "second attempt",
			attempt:      1,
			initialDelay: 1 * time.Second,
			backoff:      2.0,
			maxDelay:     10 * time.Second,
			expected:     1 * time.Second,
		},
		{
			name:         "third attempt",
			attempt:      2,
			initialDelay: 1 * time.Second,
			backoff:      2.0,
			maxDelay:     10 * time.Second,
			expected:     2 * time.Second,
		},
		{
			name:         "capped at max delay",
			attempt:      10,
			initialDelay: 1 * time.Second,
			backoff:      2.0,
			maxDelay:     5 * time.Second,
			expected:     5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := CalculateDelay(tt.attempt, tt.initialDelay, tt.backoff, tt.maxDelay)
			if delay != tt.expected {
				t.Errorf("Expected delay %v, got %v", tt.expected, delay)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "actual error",
			err:      errors.New("test error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNoRetry(t *testing.T) {
	err := NoRetry()
	if err == nil {
		t.Error("Expected error, got nil")
	}

	if err.Error() != "no retry" {
		t.Errorf("Expected 'no retry', got '%s'", err.Error())
	}
}
