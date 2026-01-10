package config

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/yourname/lotty/pkg/types"
	"github.com/spf13/viper"
)

// LoadConfig loads configuration from command line args, environment variables, and defaults
func LoadConfig(cfgFile string) (*types.Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("buffer-size", 10000)
	v.SetDefault("batch-interval", "900ms")
	v.SetDefault("batch-size", 1000)
	v.SetDefault("retry-count", 3)
	v.SetDefault("input-mode", "pty")
	v.SetDefault("log-level", "info")
	v.SetDefault("pty-rows", 24)
	v.SetDefault("pty-cols", 80)
	v.SetDefault("stats-interval", "30s")
	v.SetDefault("tls-skip-verify", false)

	// Environment variables
	v.BindEnv("loki-endpoint", "LOKI_ENDPOINT")
	v.BindEnv("loki-username", "LOKI_USERNAME")
	v.BindEnv("loki-password", "LOKI_PASSWORD")
	v.BindEnv("buffer-size", "BUFFER_SIZE")
	v.BindEnv("batch-interval", "BATCH_INTERVAL")
	v.BindEnv("retry-count", "RETRY_COUNT")
	v.BindEnv("input-mode", "INPUT_MODE")
	v.BindEnv("log-level", "LOG_LEVEL")
	v.BindEnv("pty-rows", "PTY_ROWS")
	v.BindEnv("pty-cols", "PTY_COLS")
	v.BindEnv("stats-interval", "STATS_INTERVAL")
	v.BindEnv("tls-skip-verify", "TLS_SKIP_VERIFY")

	// Read config file if specified
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("error reading config file %s: %w", cfgFile, err)
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
		PTYRows:       v.GetInt("pty-rows"),
		PTYCols:       v.GetInt("pty-cols"),
		TLSSkipVerify:  v.GetBool("tls-skip-verify"),
	}

	// Parse batch interval
	if batchInterval := v.GetString("batch-interval"); batchInterval != "" {
		duration, err := time.ParseDuration(batchInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid batch-interval: %w", err)
		}
		cfg.BatchInterval = duration
	}

	// Parse stats interval
	if statsInterval := v.GetString("stats-interval"); statsInterval != "" {
		duration, err := time.ParseDuration(statsInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid stats-interval: %w", err)
		}
		cfg.StatsInterval = duration
	}

	// Validate config
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseLabels parses comma-separated key=value pairs into a map
//
// Loki Label Requirements:
// - Label names must match the regex: [a-zA-Z_:][a-zA-Z0-9_:]*
//   - Must start with a letter or underscore
//   - Can contain letters, numbers, underscores, or colons
// - Label values must be non-empty strings
// - Avoid using reserved Loki label names like "level", "job", "instance"
//   unless you specifically want to override them
//
// Examples of valid labels:
//   "service=api,env=production,version=1.0.0"
//   "app=myapp,region=us-east-1"
//
// Examples of invalid labels:
//   "123label=value" (starts with a number)
//   "label name=value" (contains spaces)
//   "empty=" (empty value)
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

	// Validate endpoint URL format
	if err := validateURL(cfg.LokiEndpoint); err != nil {
		return fmt.Errorf("invalid loki-endpoint: %w", err)
	}

	if cfg.BufferSize <= 0 {
		return fmt.Errorf("buffer-size must be positive")
	}

	// Add reasonable upper limit for buffer size
	if cfg.BufferSize > 1000000 {
		return fmt.Errorf("buffer-size too large (max: 1000000)")
	}

	if cfg.BatchInterval <= 0 {
		return fmt.Errorf("batch-interval must be positive")
	}

	// Batch interval should not be too short
	if cfg.BatchInterval < 100*time.Millisecond {
		return fmt.Errorf("batch-interval too short (minimum: 100ms)")
	}

	if cfg.BatchSize <= 0 {
		return fmt.Errorf("batch-size must be positive")
	}

	// Validate batch size doesn't exceed buffer size
	if cfg.BatchSize > cfg.BufferSize {
		return fmt.Errorf("batch-size cannot exceed buffer-size")
	}

	if cfg.RetryCount < 0 {
		return fmt.Errorf("retry-count must be non-negative")
	}

	// Limit retry count to reasonable value
	if cfg.RetryCount > 10 {
		return fmt.Errorf("retry-count too large (max: 10)")
	}

	if cfg.InputMode != "pty" && cfg.InputMode != "pipe" {
		return fmt.Errorf("input-mode must be 'pty' or 'pipe'")
	}

	// Validate log level
	if err := validateLogLevel(cfg.LogLevel); err != nil {
		return fmt.Errorf("invalid log-level: %w", err)
	}

	// Validate PTY size
	if cfg.PTYRows <= 0 {
		return fmt.Errorf("pty-rows must be positive")
	}
	if cfg.PTYCols <= 0 {
		return fmt.Errorf("pty-cols must be positive")
	}

	// Validate stats interval
	if cfg.StatsInterval <= 0 {
		return fmt.Errorf("stats-interval must be positive")
	}

	// Validate Loki labels
	if cfg.LokiLabels != nil {
		if err := validateLabels(cfg.LokiLabels); err != nil {
			return fmt.Errorf("invalid loki-labels: %w", err)
		}
	}

	return nil
}

// validateURL validates if a string is a valid URL
func validateURL(urlStr string) error {
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return fmt.Errorf("must start with http:// or https://")
	}
	return nil
}

// validateLogLevel validates the log level
func validateLogLevel(level string) error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if level == "" {
		return fmt.Errorf("log-level is required")
	}

	if !validLevels[level] {
		return fmt.Errorf("must be one of: debug, info, warn, error")
	}

	return nil
}

// validateLabels validates Loki labels
func validateLabels(labels map[string]string) error {
	for key, value := range labels {
		// Label keys cannot be empty
		if key == "" {
			return fmt.Errorf("label key cannot be empty")
		}

		// Label keys must match Loki's label name requirements
		// Must match regex: [a-zA-Z_:][a-zA-Z0-9_:]*
		if !isValidLabelKey(key) {
			return fmt.Errorf("invalid label key '%s': must start with a letter or underscore and contain only letters, numbers, underscores, or colons", key)
		}

		// Label values cannot be empty
		if value == "" {
			return fmt.Errorf("label value for key '%s' cannot be empty", key)
		}

		// Label values should not contain certain characters
		if strings.ContainsAny(value, "\"{}") {
			return fmt.Errorf("label value for key '%s' cannot contain quotes or braces", key)
		}
	}

	return nil
}

// isValidLabelKey checks if a string is a valid Loki label key
func isValidLabelKey(key string) bool {
	if len(key) == 0 {
		return false
	}

	// First character must be letter or underscore
	firstChar := key[0]
	if !((firstChar >= 'a' && firstChar <= 'z') ||
		(firstChar >= 'A' && firstChar <= 'Z') ||
		firstChar == '_' || firstChar == ':') {
		return false
	}

	// Remaining characters can be letters, numbers, underscores, or colons
	for _, ch := range key[1:] {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '_' || ch == ':') {
			return false
		}
	}

	return true
}

// SetCommand sets the command and arguments to execute
func SetCommand(cfg *types.Config, cmd string, args ...string) error {
	// Validate command exists
	if err := validateCommand(cmd); err != nil {
		return err
	}

	cfg.Command = cmd
	cfg.Args = args
	return nil
}

// validateCommand checks if a command exists and is executable
func validateCommand(cmd string) error {
	if cmd == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// Look for command in PATH
	path, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("command '%s' not found in PATH: %w", cmd, err)
	}

	// Check if it's executable
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access command '%s': %w", cmd, err)
	}

	// Check if it's a regular file and executable
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("command '%s' is not executable", cmd)
	}

	return nil
}
