package types

import "time"

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time
	Line      string
}

// LokiStream represents a Loki stream with labels and values
type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string         `json:"values"`
}

// BatchRequest represents the request body for Loki push API
type BatchRequest struct {
	Streams []LokiStream `json:"streams"`
}

// Config holds all configuration for the application
type Config struct {
	LokiEndpoint   string            `mapstructure:"loki-endpoint"`
	LokiUsername   string            `mapstructure:"loki-username"`
	LokiPassword   string            `mapstructure:"loki-password"`
	LokiLabels     map[string]string `mapstructure:"loki-labels"`
	BufferSize     int               `mapstructure:"buffer-size"`
	BatchInterval  time.Duration     `mapstructure:"batch-interval"`
	BatchSize      int               `mapstructure:"batch-size"`
	RetryCount     int               `mapstructure:"retry-count"`
	InputMode      string            `mapstructure:"input-mode"`
	LogLevel       string            `mapstructure:"log-level"`
	Command        string            `mapstructure:"command"`
	Args           []string          `mapstructure:"args"`
	PTYRows        int               `mapstructure:"pty-rows"`
	PTYCols        int               `mapstructure:"pty-cols"`
	StatsInterval  time.Duration     `mapstructure:"stats-interval"`
	TLSSkipVerify  bool              `mapstructure:"tls-skip-verify"`
}

// BufferStats holds buffer statistics
type BufferStats struct {
	Size       int
	Capacity   int
	Dropped    int64
	Enqueued   int64
	Dequeued   int64
}

// Metrics holds application metrics
type Metrics struct {
	MessagesSent     int64
	MessagesDropped  int64
	HTTPRequests     int64
	HTTPFailures     int64
	RetryAttempts    int64
}

// Reset resets all metrics to zero
func (m *Metrics) Reset() {
	m.MessagesSent = 0
	m.MessagesDropped = 0
	m.HTTPRequests = 0
	m.HTTPFailures = 0
	m.RetryAttempts = 0
}
