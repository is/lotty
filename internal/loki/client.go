package loki

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourname/lotty/internal/retry"
	"github.com/yourname/lotty/pkg/types"
)

// Client is an HTTP client for sending logs to Loki
type Client struct {
	endpoint       string
	httpClient     *http.Client
	auth           *Auth
	logger         *logrus.Logger
	requestTimeout time.Duration
	tlsSkipVerify  bool
}

// NewClient creates a new Loki client
func NewClient(endpoint string, timeout time.Duration, logger *logrus.Logger) *Client {
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:         logger,
		requestTimeout: timeout / 2, // Request timeout is half of connection timeout
		tlsSkipVerify:  false,
	}
}

// SetTLSConfig configures TLS settings
func (c *Client) SetTLSConfig(skipVerify bool) {
	c.tlsSkipVerify = skipVerify

	if skipVerify {
		// Create custom transport that skips TLS verification
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		c.httpClient.Transport = transport
		c.logger.Warn("TLS certificate verification is disabled (INSECURE)")
	} else {
		// Reset to default transport
		c.httpClient.Transport = nil
	}
}

// SetAuth sets the authentication credentials
func (c *Client) SetAuth(username, password string) {
	c.auth = NewAuth(username, password)
}

// Push sends a batch of log entries to Loki
func (c *Client) Push(streams []types.LokiStream) error {
	// Build request body
	reqBody := types.BatchRequest{
		Streams: streams,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create context with timeout for this specific request
	ctx, cancel := context.WithTimeout(context.Background(), c.requestTimeout)
	defer cancel()

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.auth != nil {
		c.auth.SetHeader(req)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("request timeout after %v", c.requestTimeout)
		}
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Debugf("Successfully sent batch to Loki (status: %d)", resp.StatusCode)
	return nil
}

// PushWithRetry sends a batch of log entries to Loki with retry logic
func (c *Client) PushWithRetry(streams []types.LokiStream, retryCount int, initialRetryDelay time.Duration) error {
	ctx := context.Background()
	config := &retry.RetryConfig{
		MaxRetries:    retryCount,
		InitialDelay:  initialRetryDelay,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}

	fn := func() error {
		return c.Push(streams)
	}

	return retry.Retry(ctx, fn, config, c.logger)
}

// Close closes the HTTP client
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// BuildLokiStream builds a Loki stream from log entries and labels
func BuildLokiStream(entries []types.LogEntry, labels map[string]string) types.LokiStream {
	values := make([][]string, len(entries))
	for i, entry := range entries {
		// Loki expects nanosecond timestamp as string (Unix timestamp in nanoseconds)
		timestamp := fmt.Sprintf("%d", entry.Timestamp.UnixNano())
		values[i] = []string{timestamp, entry.Line}
	}

	return types.LokiStream{
		Stream: labels,
		Values: values,
	}
}
