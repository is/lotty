package loki

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourname/loky/pkg/types"
)

func TestNewClient(t *testing.T) {
	logger := logrus.New()
	client := NewClient("http://localhost:3100/loki/api/v1/push", 30*time.Second, logger)

	if client.endpoint != "http://localhost:3100/loki/api/v1/push" {
		t.Errorf("Expected endpoint 'http://localhost:3100/loki/api/v1/push', got '%s'", client.endpoint)
	}

	if client.httpClient == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestAuth_SetHeader(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "with credentials",
			username: "testuser",
			password: "testpass",
		},
		{
			name:     "empty credentials",
			username: "",
			password: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewAuth(tt.username, tt.password)
			req := httptest.NewRequest("POST", "http://example.com", nil)
			auth.SetHeader(req)

			if tt.username != "" || tt.password != "" {
				authHeader := req.Header.Get("Authorization")
				if authHeader == "" {
					t.Error("Expected Authorization header to be set")
				}
			}
		})
	}
}

func TestAuth_Validate(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{
			name:     "with username",
			username: "testuser",
			password: "",
			wantErr:  false,
		},
		{
			name:     "with password",
			username: "",
			password: "testpass",
			wantErr:  false,
		},
		{
			name:     "with both",
			username: "testuser",
			password: "testpass",
			wantErr:  false,
		},
		{
			name:     "empty",
			username: "",
			password: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewAuth(tt.username, tt.password)
			err := auth.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_Push(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress debug output

	client := NewClient(server.URL, 30*time.Second, logger)

	entry := types.LogEntry{
		Timestamp: time.Now(),
		Line:      "test log line",
	}

	entries := []types.LogEntry{entry}
	labels := map[string]string{"service": "test"}
	stream := BuildLokiStream(entries, labels)
	streams := []types.LokiStream{stream}

	err := client.Push(streams)
	if err != nil {
		t.Errorf("Push() error = %v", err)
	}
}

func TestClient_Push_Error(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad request"))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	client := NewClient(server.URL, 30*time.Second, logger)

	entry := types.LogEntry{
		Timestamp: time.Now(),
		Line:      "test log line",
	}

	entries := []types.LogEntry{entry}
	labels := map[string]string{"service": "test"}
	stream := BuildLokiStream(entries, labels)
	streams := []types.LokiStream{stream}

	err := client.Push(streams)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestBuildLokiStream(t *testing.T) {
	now := time.Now()
	entries := []types.LogEntry{
		{
			Timestamp: now,
			Line:      "line1",
		},
		{
			Timestamp: now,
			Line:      "line2",
		},
	}

	labels := map[string]string{"service": "test", "env": "dev"}

	stream := BuildLokiStream(entries, labels)

	if len(stream.Values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(stream.Values))
	}

	if len(stream.Stream) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(stream.Stream))
	}

	if stream.Stream["service"] != "test" {
		t.Errorf("Expected service=test, got service=%s", stream.Stream["service"])
	}

	if len(stream.Values[0]) != 2 {
		t.Errorf("Expected 2 values per entry, got %d", len(stream.Values[0]))
	}
}

func TestClient_PushWithRetry(t *testing.T) {
	attemptCount := 0

	// Create test server that fails first 2 times
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	client := NewClient(server.URL, 30*time.Second, logger)

	entry := types.LogEntry{
		Timestamp: time.Now(),
		Line:      "test log line",
	}

	entries := []types.LogEntry{entry}
	labels := map[string]string{"service": "test"}
	stream := BuildLokiStream(entries, labels)
	streams := []types.LokiStream{stream}

	err := client.PushWithRetry(streams, 3, 10*time.Millisecond)
	if err != nil {
		t.Errorf("PushWithRetry() error = %v", err)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

func TestClient_PushWithRetry_MaxRetries(t *testing.T) {
	// Create test server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	client := NewClient(server.URL, 30*time.Second, logger)

	entry := types.LogEntry{
		Timestamp: time.Now(),
		Line:      "test log line",
	}

	entries := []types.LogEntry{entry}
	labels := map[string]string{"service": "test"}
	stream := BuildLokiStream(entries, labels)
	streams := []types.LokiStream{stream}

	err := client.PushWithRetry(streams, 2, 10*time.Millisecond)
	if err == nil {
		t.Error("Expected error after max retries, got nil")
	}
}
