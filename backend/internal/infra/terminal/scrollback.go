package terminal

import (
	"bytes"
	"sync"
)

// ScrollbackBuffer maintains a circular buffer of terminal output
// for reconnection support
type ScrollbackBuffer struct {
	mu       sync.RWMutex
	chunks   [][]byte
	size     int
	maxSize  int
}

// NewScrollbackBuffer creates a new scrollback buffer with the given max size
func NewScrollbackBuffer(maxSize int) *ScrollbackBuffer {
	return &ScrollbackBuffer{
		chunks:  make([][]byte, 0),
		maxSize: maxSize,
	}
}

// Write adds data to the buffer, trimming if necessary
func (b *ScrollbackBuffer) Write(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Make a copy of the data
	chunk := make([]byte, len(data))
	copy(chunk, data)
	b.chunks = append(b.chunks, chunk)
	b.size += len(chunk)

	// Trim buffer if too large
	for b.size > b.maxSize && len(b.chunks) > 0 {
		removed := b.chunks[0]
		b.chunks = b.chunks[1:]
		b.size -= len(removed)
	}
}

// GetAll returns all data in the buffer
func (b *ScrollbackBuffer) GetAll() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.chunks) == 0 {
		return nil
	}

	result := make([]byte, 0, b.size)
	for _, chunk := range b.chunks {
		result = append(result, chunk...)
	}
	return result
}

// GetLastNLines returns the last N lines from the buffer
func (b *ScrollbackBuffer) GetLastNLines(n int) []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.chunks) == 0 {
		return nil
	}

	// Join all chunks
	allData := make([]byte, 0, b.size)
	for _, chunk := range b.chunks {
		allData = append(allData, chunk...)
	}

	// Split by newlines and get last N
	lines := bytes.Split(allData, []byte{'\n'})
	if len(lines) <= n {
		return allData
	}

	return bytes.Join(lines[len(lines)-n:], []byte{'\n'})
}

// Clear clears the buffer
func (b *ScrollbackBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chunks = make([][]byte, 0)
	b.size = 0
}

// Size returns the current size of the buffer
func (b *ScrollbackBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.size
}
