package retry

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/yourname/loky/pkg/types"
)

// RetryConfig holds the retry configuration
type RetryConfig struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// RetryFunc is a function that can be retried
type RetryFunc func() error

// RetryLogger is an interface for logging retry attempts
type RetryLogger interface {
	Debugf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, fn RetryFunc, config *RetryConfig, logger RetryLogger) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Check for context cancellation
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// Log retry attempt
			if logger != nil {
				logger.Debugf("Retry attempt %d/%d after %v", attempt, config.MaxRetries, delay)
			}

			// Wait before retry
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}

			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * config.BackoffFactor)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if logger != nil {
			logger.Warnf("Attempt %d failed: %v", attempt, err)
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// RetryWithMetrics executes a function with retry logic and updates metrics
func RetryWithMetrics(ctx context.Context, fn RetryFunc, config *RetryConfig, logger RetryLogger, metrics *types.Metrics) error {
	err := Retry(ctx, fn, config, logger)
	if err != nil {
		if metrics != nil {
			metrics.HTTPFailures++
		}
	}
	return err
}

// CalculateDelay calculates the delay for a given retry attempt
func CalculateDelay(attempt int, initialDelay time.Duration, backoffFactor float64, maxDelay time.Duration) time.Duration {
	if attempt == 0 {
		return 0
	}

	delay := time.Duration(float64(initialDelay) * math.Pow(backoffFactor, float64(attempt-1)))
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

// IsRetryableError checks if an error should trigger a retry
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Add specific error checks here
	// For now, all errors are retryable
	return true
}

// NoRetry is a helper function that always returns an error to prevent retries
func NoRetry() error {
	return fmt.Errorf("no retry")
}
