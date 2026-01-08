// Package buffer provides buffer utilities.
package buffer

import "sync"

// Ring is a circular buffer for storing recent data.
// It is thread-safe and efficiently stores the most recent N bytes,
// discarding older data as new data is written.
type Ring struct {
	data  []byte
	size  int
	start int
	end   int
	full  bool
	mu    sync.Mutex
}

// NewRing creates a new ring buffer with the specified size.
func NewRing(size int) *Ring {
	return &Ring{
		data: make([]byte, size),
		size: size,
	}
}

// Write writes data to the ring buffer.
// Implements io.Writer interface.
func (rb *Ring) Write(p []byte) (n int, err error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for _, b := range p {
		rb.data[rb.end] = b
		rb.end = (rb.end + 1) % rb.size
		if rb.full {
			rb.start = (rb.start + 1) % rb.size
		}
		if rb.end == rb.start {
			rb.full = true
		}
	}
	return len(p), nil
}

// Bytes returns all data in the buffer.
// The returned slice is a copy of the internal data.
func (rb *Ring) Bytes() []byte {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if !rb.full && rb.start == rb.end {
		return nil
	}

	if rb.full {
		result := make([]byte, rb.size)
		copy(result, rb.data[rb.start:])
		copy(result[rb.size-rb.start:], rb.data[:rb.end])
		return result
	}

	if rb.end > rb.start {
		result := make([]byte, rb.end-rb.start)
		copy(result, rb.data[rb.start:rb.end])
		return result
	}

	return nil
}

// Reset clears the buffer.
func (rb *Ring) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.start = 0
	rb.end = 0
	rb.full = false
}

// Len returns the current number of bytes in the buffer.
func (rb *Ring) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.full {
		return rb.size
	}
	if rb.end >= rb.start {
		return rb.end - rb.start
	}
	return rb.size - rb.start + rb.end
}

// Cap returns the capacity of the buffer.
func (rb *Ring) Cap() int {
	return rb.size
}
