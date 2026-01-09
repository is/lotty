package buffer

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourname/loky/pkg/types"
)

// RingBuffer implements a thread-safe ring buffer for log entries
type RingBuffer struct {
	mu        sync.RWMutex
	buffer    []types.LogEntry
	head      int
	tail      int
	size      int
	capacity  int
	stats     types.BufferStats
	logger    *logrus.Logger
	dropChan  chan struct{}
	fullChan  chan struct{}
}

// NewRingBuffer creates a new RingBuffer with the specified capacity
func NewRingBuffer(capacity int, logger *logrus.Logger) *RingBuffer {
	return &RingBuffer{
		buffer:   make([]types.LogEntry, capacity),
		capacity: capacity,
		stats: types.BufferStats{
			Capacity: capacity,
		},
		logger:   logger,
		dropChan: make(chan struct{}, 100),
		fullChan: make(chan struct{}, 1),
	}
}

// Enqueue adds a log entry to the buffer
func (rb *RingBuffer) Enqueue(entry types.LogEntry) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == rb.capacity {
		// Buffer is full, drop the oldest message (FIFO)
		rb.dropOldest()
		rb.stats.Dropped++
		rb.dropChan <- struct{}{}
		rb.logger.Debugf("Buffer full, dropped oldest message (total dropped: %d)", rb.stats.Dropped)
	}

	rb.buffer[rb.tail] = entry
	rb.tail = (rb.tail + 1) % rb.capacity
	rb.size++
	rb.stats.Enqueued++

	// Notify if buffer is full
	if rb.size == rb.capacity {
		select {
		case rb.fullChan <- struct{}{}:
		default:
		}
	}

	return true
}

// Dequeue removes and returns a log entry from the buffer
func (rb *RingBuffer) Dequeue() (types.LogEntry, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return types.LogEntry{}, false
	}

	entry := rb.buffer[rb.head]
	rb.buffer[rb.head] = types.LogEntry{}
	rb.head = (rb.head + 1) % rb.capacity
	rb.size--
	rb.stats.Dequeued++

	return entry, true
}

// DequeueBatch removes and returns up to n log entries from the buffer
func (rb *RingBuffer) DequeueBatch(n int) []types.LogEntry {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if n > rb.size {
		n = rb.size
	}

	if n == 0 {
		return []types.LogEntry{}
	}

	entries := make([]types.LogEntry, n)
	for i := 0; i < n; i++ {
		entries[i] = rb.buffer[rb.head]
		rb.buffer[rb.head] = types.LogEntry{}
		rb.head = (rb.head + 1) % rb.capacity
	}

	rb.size -= n
	rb.stats.Dequeued += int64(n)

	return entries
}

// DequeueAll removes and returns all log entries from the buffer
func (rb *RingBuffer) DequeueAll() []types.LogEntry {
	return rb.DequeueBatch(rb.Size())
}

// Size returns the current number of entries in the buffer
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// Capacity returns the buffer capacity
func (rb *RingBuffer) Capacity() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.capacity
}

// IsFull returns true if the buffer is full
func (rb *RingBuffer) IsFull() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size == rb.capacity
}

// IsEmpty returns true if the buffer is empty
func (rb *RingBuffer) IsEmpty() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size == 0
}

// Stats returns the buffer statistics
func (rb *RingBuffer) Stats() types.BufferStats {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	stats := rb.stats
	stats.Size = rb.size
	return stats
}

// Reset clears the buffer and resets statistics
func (rb *RingBuffer) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer = make([]types.LogEntry, rb.capacity)
	rb.head = 0
	rb.tail = 0
	rb.size = 0
	rb.stats = types.BufferStats{
		Capacity: rb.capacity,
	}
}

// dropOldest removes the oldest message from the buffer (FIFO)
func (rb *RingBuffer) dropOldest() {
	rb.buffer[rb.head] = types.LogEntry{}
	rb.head = (rb.head + 1) % rb.capacity
	rb.size--
}

// DropChannel returns a channel that receives a notification when a message is dropped
func (rb *RingBuffer) DropChannel() <-chan struct{} {
	return rb.dropChan
}

// FullChannel returns a channel that receives a notification when the buffer becomes full
func (rb *RingBuffer) FullChannel() <-chan struct{} {
	return rb.fullChan
}

// CreateLogEntry creates a new log entry with the current timestamp
func CreateLogEntry(line string) types.LogEntry {
	return types.LogEntry{
		Timestamp: time.Now(),
		Line:      line,
	}
}
