package batcher

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourname/loky/internal/buffer"
	"github.com/yourname/loky/internal/loki"
	"github.com/yourname/loky/pkg/types"
)

type trackingLogger struct {
	infoMessages  []string
	debugMessages []string
	errorMessages []string
}

func (t *trackingLogger) Debugf(format string, args ...interface{}) {
	t.debugMessages = append(t.debugMessages, format)
}

func (t *trackingLogger) Infof(format string, args ...interface{}) {
	t.infoMessages = append(t.infoMessages, format)
}

func (t *trackingLogger) Warnf(format string, args ...interface{}) {
	t.errorMessages = append(t.errorMessages, format)
}

func (t *trackingLogger) Errorf(format string, args ...interface{}) {
	t.errorMessages = append(t.errorMessages, format)
}

func TestNewBatcher(t *testing.T) {
	buf := buffer.NewRingBuffer(100, logrus.New())
	client := loki.NewClient("http://localhost:3100", 30*time.Second, logrus.New())
	logger := &trackingLogger{}
	labels := map[string]string{"service": "test"}

	b := NewBatcher(buf, client, labels, 900*time.Millisecond, 100, 3, logger)

	if b == nil {
		t.Fatal("Expected non-nil batcher")
	}

	if b.batchInterval != 900*time.Millisecond {
		t.Errorf("Expected batch interval 900ms, got %v", b.batchInterval)
	}

	if b.batchSize != 100 {
		t.Errorf("Expected batch size 100, got %d", b.batchSize)
	}

	if b.retryCount != 3 {
		t.Errorf("Expected retry count 3, got %d", b.retryCount)
	}
}

func TestBatcher_StartAndStop(t *testing.T) {
	buf := buffer.NewRingBuffer(100, logrus.New())
	client := loki.NewClient("http://localhost:3100", 30*time.Second, logrus.New())
	logger := &trackingLogger{}
	labels := map[string]string{"service": "test"}

	b := NewBatcher(buf, client, labels, 900*time.Millisecond, 100, 3, logger)
	b.Start()

	time.Sleep(100 * time.Millisecond)

	b.Stop()

	// Just verify it doesn't crash
}

func TestBatcher_EmptyBuffer(t *testing.T) {
	buf := buffer.NewRingBuffer(100, logrus.New())
	client := loki.NewClient("http://localhost:3100", 30*time.Second, logrus.New())
	logger := &trackingLogger{}
	labels := map[string]string{"service": "test"}

	b := NewBatcher(buf, client, labels, 200*time.Millisecond, 100, 3, logger)
	b.Start()

	// Don't add any entries
	time.Sleep(500 * time.Millisecond)

	b.Stop()

	metrics := b.GetMetrics()
	if metrics.HTTPRequests != 0 {
		t.Error("Expected 0 HTTP requests for empty buffer")
	}
}

func TestBuildLokiStream(t *testing.T) {
	entries := []types.LogEntry{
		{
			Timestamp: time.Now(),
			Line:      "line1",
		},
		{
			Timestamp: time.Now(),
			Line:      "line2",
		},
	}

	labels := map[string]string{"service": "test", "env": "dev"}

	stream := loki.BuildLokiStream(entries, labels)

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
