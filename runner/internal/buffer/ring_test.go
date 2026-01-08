package buffer

import (
	"bytes"
	"sync"
	"testing"
)

func TestNewRing(t *testing.T) {
	rb := NewRing(100)

	if rb == nil {
		t.Fatal("NewRing returned nil")
	}

	if rb.Cap() != 100 {
		t.Errorf("Cap: got %v, want 100", rb.Cap())
	}

	if rb.Len() != 0 {
		t.Errorf("Len: got %v, want 0", rb.Len())
	}
}

func TestRingWrite(t *testing.T) {
	rb := NewRing(10)

	n, err := rb.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != 5 {
		t.Errorf("n: got %v, want 5", n)
	}

	if rb.Len() != 5 {
		t.Errorf("Len: got %v, want 5", rb.Len())
	}
}

func TestRingBytes(t *testing.T) {
	rb := NewRing(10)

	rb.Write([]byte("hello"))

	data := rb.Bytes()
	if !bytes.Equal(data, []byte("hello")) {
		t.Errorf("Bytes: got %v, want hello", string(data))
	}
}

func TestRingOverflow(t *testing.T) {
	rb := NewRing(5)

	rb.Write([]byte("abcdefgh"))

	data := rb.Bytes()
	// Should only have last 5 bytes: "defgh"
	if !bytes.Equal(data, []byte("defgh")) {
		t.Errorf("Bytes: got %v, want defgh", string(data))
	}

	if rb.Len() != 5 {
		t.Errorf("Len: got %v, want 5", rb.Len())
	}
}

func TestRingReset(t *testing.T) {
	rb := NewRing(10)

	rb.Write([]byte("hello"))
	rb.Reset()

	if rb.Len() != 0 {
		t.Errorf("Len after reset: got %v, want 0", rb.Len())
	}

	data := rb.Bytes()
	if data != nil {
		t.Errorf("Bytes after reset: got %v, want nil", data)
	}
}

func TestRingEmptyBytes(t *testing.T) {
	rb := NewRing(10)

	data := rb.Bytes()
	if data != nil {
		t.Errorf("Bytes on empty buffer: got %v, want nil", data)
	}
}

func TestRingCap(t *testing.T) {
	rb := NewRing(50)

	if rb.Cap() != 50 {
		t.Errorf("Cap: got %v, want 50", rb.Cap())
	}
}

func TestRingWrapAround(t *testing.T) {
	rb := NewRing(5)

	// Write more than capacity to ensure wrap-around
	rb.Write([]byte("12345"))
	if rb.Len() != 5 {
		t.Errorf("Len after fill: got %v, want 5", rb.Len())
	}

	// Write more to trigger wrap
	rb.Write([]byte("67"))

	data := rb.Bytes()
	if !bytes.Equal(data, []byte("34567")) {
		t.Errorf("Bytes after wrap: got %v, want 34567", string(data))
	}
}

func TestRingMultipleWrites(t *testing.T) {
	rb := NewRing(20)

	rb.Write([]byte("hello"))
	rb.Write([]byte(" "))
	rb.Write([]byte("world"))

	data := rb.Bytes()
	if !bytes.Equal(data, []byte("hello world")) {
		t.Errorf("Bytes: got %v, want 'hello world'", string(data))
	}
}

func TestRingFullBuffer(t *testing.T) {
	rb := NewRing(5)

	rb.Write([]byte("12345"))

	if rb.Len() != 5 {
		t.Errorf("Len: got %v, want 5", rb.Len())
	}

	data := rb.Bytes()
	if !bytes.Equal(data, []byte("12345")) {
		t.Errorf("Bytes: got %v, want 12345", string(data))
	}
}

func TestRingConcurrentWrite(t *testing.T) {
	rb := NewRing(100)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Write([]byte("x"))
			}
		}(i)
	}

	wg.Wait()

	// Buffer should be full or have some data
	if rb.Len() == 0 {
		t.Error("buffer should not be empty after concurrent writes")
	}
}

func TestRingConcurrentReadWrite(t *testing.T) {
	rb := NewRing(50)

	var wg sync.WaitGroup

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			rb.Write([]byte("data"))
		}
	}()

	// Reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = rb.Bytes()
			_ = rb.Len()
		}
	}()

	wg.Wait()
}

func TestRingIOWriter(t *testing.T) {
	rb := NewRing(100)

	// Use as io.Writer
	_, err := rb.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Bytes should match
	data := rb.Bytes()
	if !bytes.Equal(data, []byte("test")) {
		t.Errorf("Bytes: got %v, want test", string(data))
	}
}

func TestRingLenDuringOverflow(t *testing.T) {
	rb := NewRing(5)

	rb.Write([]byte("123"))
	if rb.Len() != 3 {
		t.Errorf("Len after partial fill: got %v, want 3", rb.Len())
	}

	rb.Write([]byte("4567"))
	// Buffer should be full now
	if rb.Len() != 5 {
		t.Errorf("Len after overflow: got %v, want 5", rb.Len())
	}
}

func BenchmarkRingWrite(b *testing.B) {
	rb := NewRing(1024)
	data := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Write(data)
	}
}

func BenchmarkRingBytes(b *testing.B) {
	rb := NewRing(1024)
	rb.Write([]byte("some initial data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rb.Bytes()
	}
}

func BenchmarkRingLen(b *testing.B) {
	rb := NewRing(1024)
	rb.Write([]byte("some initial data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rb.Len()
	}
}
