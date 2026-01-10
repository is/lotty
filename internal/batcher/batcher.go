package batcher

import (
	"context"
	"sync"
	"time"

	"github.com/yourname/lotty/internal/buffer"
	"github.com/yourname/lotty/internal/loki"
	"github.com/yourname/lotty/pkg/types"
)

// Batcher handles batching and sending log entries to Loki
type Batcher struct {
	buffer         *buffer.RingBuffer
	client         *loki.Client
	labels         map[string]string
	batchInterval  time.Duration
	batchSize      int
	retryCount     int
	retryDelay     time.Duration
	logger         Logger
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	metrics        *types.Metrics
	mu             sync.Mutex
	forceSend      chan struct{}
}

// Logger interface for logging
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// NewBatcher creates a new Batcher
func NewBatcher(
	buf *buffer.RingBuffer,
	client *loki.Client,
	labels map[string]string,
	batchInterval time.Duration,
	batchSize int,
	retryCount int,
	logger Logger,
) *Batcher {
	ctx, cancel := context.WithCancel(context.Background())

	return &Batcher{
		buffer:        buf,
		client:        client,
		labels:        labels,
		batchInterval: batchInterval,
		batchSize:     batchSize,
		retryCount:    retryCount,
		retryDelay:    1 * time.Second,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		metrics:       &types.Metrics{},
		forceSend:     make(chan struct{}, 1),
	}
}

// Start starts the batcher
func (b *Batcher) Start() {
	b.wg.Add(1)
	go b.run()
}

// Stop stops the batcher gracefully
func (b *Batcher) Stop() {
	b.cancel()
	b.wg.Wait()
}

// Flush forces a flush of the buffer
func (b *Batcher) Flush() {
	select {
	case b.forceSend <- struct{}{}:
	default:
	}
}

// GetMetrics returns the current metrics
func (b *Batcher) GetMetrics() types.Metrics {
	b.mu.Lock()
	defer b.mu.Unlock()
	return *b.metrics
}

// run is the main batcher loop
func (b *Batcher) run() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.batchInterval)
	defer ticker.Stop()

	fullChan := b.buffer.FullChannel()

	for {
		select {
		case <-b.ctx.Done():
			// Final flush before exiting
			b.flush()
			return

		case <-ticker.C:
			// Time-based flush
			b.flush()

		case <-fullChan:
			// Buffer is full, flush immediately
			b.flush()

		case <-b.forceSend:
			// Force flush requested
			b.flush()
		}
	}
}

// flush sends all pending log entries to Loki
func (b *Batcher) flush() {
	// Dequeue all entries
	entries := b.buffer.DequeueAll()

	if len(entries) == 0 {
		return
	}

	b.logger.Debugf("Flushing %d log entries to Loki", len(entries))

	// Send in batches
	for i := 0; i < len(entries); i += b.batchSize {
		end := i + b.batchSize
		if end > len(entries) {
			end = len(entries)
		}

		batch := entries[i:end]
		if err := b.sendBatch(batch); err != nil {
			b.logger.Errorf("Failed to send batch: %v", err)
			b.mu.Lock()
			b.metrics.MessagesDropped += int64(len(batch))
			b.mu.Unlock()
		}
	}
}

// sendBatch sends a batch of log entries to Loki
func (b *Batcher) sendBatch(entries []types.LogEntry) error {
	// Build Loki stream
	stream := loki.BuildLokiStream(entries, b.labels)
	streams := []types.LokiStream{stream}

	// Send with retry
	err := b.client.PushWithRetry(streams, b.retryCount, b.retryDelay)

	if err != nil {
		b.mu.Lock()
		b.metrics.HTTPFailures++
		b.mu.Unlock()
		return err
	}

	// Update metrics
	b.mu.Lock()
	b.metrics.MessagesSent += int64(len(entries))
	b.metrics.HTTPRequests++
	b.mu.Unlock()

	b.logger.Debugf("Successfully sent %d log entries", len(entries))
	return nil
}

// MonitorStats periodically logs buffer statistics
func (b *Batcher) MonitorStats(interval time.Duration) {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-b.ctx.Done():
				return
			case <-ticker.C:
				stats := b.buffer.Stats()
				b.logger.Debugf("Buffer stats: %d/%d (dropped: %d)", stats.Size, stats.Capacity, stats.Dropped)
			}
		}
	}()
}
