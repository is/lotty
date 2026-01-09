package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yourname/lotty/pkg/types"
	"github.com/spf13/viper"
)

// LoadConfig loads configuration from command line args, environment variables, and defaults
func LoadConfig() (*types.Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("buffer-size", 10000)
	v.SetDefault("batch-interval", "900ms")
	v.SetDefault("batch-size", 1000)
	v.SetDefault("retry-count", 3)
	v.SetDefault("input-mode", "pty")
	v.SetDefault("log-level", "info")

	// Environment variables
	v.SetEnvPrefix("")
	v.AutomaticEnv()
	v.BindEnv("loki-endpoint", "LOKI_ENDPOINT")
	v.BindEnv("loki-username", "LOKI_USERNAME")
	v.BindEnv("loki-password", "LOKI_PASSWORD")
	v.BindEnv("buffer-size", "BUFFER_SIZE")
	v.BindEnv("batch-interval", "BATCH_INTERVAL")
	v.BindEnv("retry-count", "RETRY_COUNT")
	v.BindEnv("input-mode", "INPUT_MODE")
	v.BindEnv("log-level", "LOG_LEVEL")

	// Read environment variables
	if err := v.ReadInConfig(); err != nil {
		// Ignore config file not found error
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
	}

	// Parse Loki labels from environment
	if labels := os.Getenv("LOKI_LABELS"); labels != "" {
		v.Set("loki-labels", parseLabels(labels))
	}

	// Create config struct
	cfg := &types.Config{
		LokiEndpoint:  v.GetString("loki-endpoint"),
		LokiUsername:  v.GetString("loki-username"),
		LokiPassword:  v.GetString("loki-password"),
		LokiLabels:    v.GetStringMapString("loki-labels"),
		BufferSize:    v.GetInt("buffer-size"),
		BatchSize:     v.GetInt("batch-size"),
		RetryCount:    v.GetInt("retry-count"),
		InputMode:     v.GetString("input-mode"),
		LogLevel:      v.GetString("log-level"),
	}

	// Parse batch interval
	if batchInterval := v.GetString("batch-interval"); batchInterval != "" {
		duration, err := time.ParseDuration(batchInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid batch-interval: %w", err)
		}
		cfg.BatchInterval = duration
	}

	// Validate config
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseLabels parses comma-separated key=value pairs into a map
func parseLabels(labels string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(labels, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

// validateConfig validates the configuration
func validateConfig(cfg *types.Config) error {
	if cfg.LokiEndpoint == "" {
		return fmt.Errorf("loki-endpoint is required")
	}
	if cfg.BufferSize <= 0 {
		return fmt.Errorf("buffer-size must be positive")
	}
	if cfg.BatchInterval <= 0 {
		return fmt.Errorf("batch-interval must be positive")
	}
	if cfg.BatchSize <= 0 {
		return fmt.Errorf("batch-size must be positive")
	}
	if cfg.RetryCount < 0 {
		return fmt.Errorf("retry-count must be non-negative")
	}
	if cfg.InputMode != "pty" && cfg.InputMode != "pipe" {
		return fmt.Errorf("input-mode must be 'pty' or 'pipe'")
	}
	return nil
}

// SetCommand sets the command and arguments to execute
func SetCommand(cfg *types.Config, cmd string, args ...string) {
	cfg.Command = cmd
	cfg.Args = args
}
