package config

import (
	"os"
	"testing"
	"time"

	"github.com/yourname/lotty/pkg/types"
)

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   string
		expected map[string]string
	}{
		{
			name:     "single label",
			labels:   "key=value",
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "multiple labels",
			labels:   "key1=value1,key2=value2",
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "empty labels",
			labels:   "",
			expected: map[string]string{},
		},
		{
			name:     "labels with spaces",
			labels:   "key1=value1, key2=value2",
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLabels(tt.labels)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d labels, got %d", len(tt.expected), len(result))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("Expected %s=%s, got %s=%s", k, v, k, result[k])
				}
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *types.Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &types.Config{
				LokiEndpoint:  "http://localhost:3100/loki/api/v1/push",
				BufferSize:    10000,
				BatchInterval: 900 * time.Millisecond,
				BatchSize:     1000,
				RetryCount:    3,
				InputMode:     "pty",
				LogLevel:      "info",
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			cfg: &types.Config{
				BufferSize:    10000,
				BatchInterval: 900 * time.Millisecond,
				BatchSize:     1000,
				RetryCount:    3,
				InputMode:     "pty",
			},
			wantErr: true,
		},
		{
			name: "invalid buffer size",
			cfg: &types.Config{
				LokiEndpoint:  "http://localhost:3100/loki/api/v1/push",
				BufferSize:    0,
				BatchInterval: 900 * time.Millisecond,
				BatchSize:     1000,
				RetryCount:    3,
				InputMode:     "pty",
			},
			wantErr: true,
		},
		{
			name: "invalid input mode",
			cfg: &types.Config{
				LokiEndpoint:  "http://localhost:3100/loki/api/v1/push",
				BufferSize:    10000,
				BatchInterval: 900 * time.Millisecond,
				BatchSize:     1000,
				RetryCount:    3,
				InputMode:     "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Set environment variables
	os.Setenv("LOKI_ENDPOINT", "http://localhost:3100/loki/api/v1/push")
	os.Setenv("LOKI_USERNAME", "testuser")
	os.Setenv("LOKI_PASSWORD", "testpass")
	os.Setenv("LOKI_LABELS", "service=test,env=dev")
	defer func() {
		os.Unsetenv("LOKI_ENDPOINT")
		os.Unsetenv("LOKI_USERNAME")
		os.Unsetenv("LOKI_PASSWORD")
		os.Unsetenv("LOKI_LABELS")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.LokiEndpoint != "http://localhost:3100/loki/api/v1/push" {
		t.Errorf("Expected endpoint %s, got %s", "http://localhost:3100/loki/api/v1/push", cfg.LokiEndpoint)
	}

	if cfg.LokiUsername != "testuser" {
		t.Errorf("Expected username testuser, got %s", cfg.LokiUsername)
	}

	if cfg.LokiPassword != "testpass" {
		t.Errorf("Expected password testpass, got %s", cfg.LokiPassword)
	}

	if len(cfg.LokiLabels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(cfg.LokiLabels))
	}

	if cfg.LokiLabels["service"] != "test" {
		t.Errorf("Expected service=test, got service=%s", cfg.LokiLabels["service"])
	}

	if cfg.BufferSize != 10000 {
		t.Errorf("Expected buffer size 10000, got %d", cfg.BufferSize)
	}

	if cfg.BatchInterval != 900*time.Millisecond {
		t.Errorf("Expected batch interval 900ms, got %v", cfg.BatchInterval)
	}
}
