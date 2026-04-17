package store

import (
	"sync"

	"github.com/potbuddy/local-node/internal/processor"
)

// RingBuffer is a thread-safe fixed-size circular buffer of EnrichedPayloads.
type RingBuffer struct {
	mu   sync.RWMutex
	buf  []processor.EnrichedPayload
	head int  // index where the next write goes
	size int  // maximum capacity
	full bool // true once the buffer has wrapped around
}

// NewRingBuffer creates a RingBuffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 100
	}
	return &RingBuffer{
		buf:  make([]processor.EnrichedPayload, size),
		size: size,
	}
}

// Push adds a new payload, overwriting the oldest entry when full.
func (rb *RingBuffer) Push(p processor.EnrichedPayload) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buf[rb.head] = p
	rb.head = (rb.head + 1) % rb.size
	if rb.head == 0 {
		rb.full = true
	}
}

// Latest returns the most recently pushed payload.
// Returns the zero value and false if the buffer is empty.
func (rb *RingBuffer) Latest() (processor.EnrichedPayload, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if !rb.full && rb.head == 0 {
		return processor.EnrichedPayload{}, false
	}
	idx := (rb.head - 1 + rb.size) % rb.size
	return rb.buf[idx], true
}

// History returns at most n entries in chronological order (oldest first).
func (rb *RingBuffer) History(n int) []processor.EnrichedPayload {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var count int
	if rb.full {
		count = rb.size
	} else {
		count = rb.head
	}
	if n > count {
		n = count
	}
	if n <= 0 {
		return nil
	}

	result := make([]processor.EnrichedPayload, n)
	// Start from the oldest entry within the requested window.
	startOffset := count - n
	for i := 0; i < n; i++ {
		idx := (rb.head - count + startOffset + i + rb.size) % rb.size
		result[i] = rb.buf[idx]
	}
	return result
}

// Len returns the number of entries currently stored.
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.full {
		return rb.size
	}
	return rb.head
}
