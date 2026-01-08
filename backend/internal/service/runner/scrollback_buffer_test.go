package runner

import (
	"bytes"
	"testing"
)

func TestNewScrollbackBuffer(t *testing.T) {
	buffer := NewScrollbackBuffer(1024)
	if buffer == nil {
		t.Fatal("NewScrollbackBuffer returned nil")
	}
	if buffer.maxSize != 1024 {
		t.Errorf("maxSize = %d, want 1024", buffer.maxSize)
	}
	if len(buffer.data) != 0 {
		t.Errorf("data length = %d, want 0", len(buffer.data))
	}
}

func TestScrollbackBufferWrite(t *testing.T) {
	buffer := NewScrollbackBuffer(100)

	buffer.Write([]byte("hello "))
	if string(buffer.data) != "hello " {
		t.Errorf("data = %q, want %q", buffer.data, "hello ")
	}

	buffer.Write([]byte("world"))
	if string(buffer.data) != "hello world" {
		t.Errorf("data = %q, want %q", buffer.data, "hello world")
	}
}

func TestScrollbackBufferWriteOverflow(t *testing.T) {
	buffer := NewScrollbackBuffer(10)

	buffer.Write([]byte("1234567890"))
	if len(buffer.data) != 10 {
		t.Errorf("data length = %d, want 10", len(buffer.data))
	}

	// Writing more should trim from the beginning
	buffer.Write([]byte("ABCDE"))
	if len(buffer.data) != 10 {
		t.Errorf("data length = %d, want 10", len(buffer.data))
	}
	// Should have last 10 bytes: "67890ABCDE"
	if string(buffer.data) != "67890ABCDE" {
		t.Errorf("data = %q, want %q", buffer.data, "67890ABCDE")
	}
}

func TestScrollbackBufferGetData(t *testing.T) {
	buffer := NewScrollbackBuffer(100)
	buffer.Write([]byte("test data"))

	data := buffer.GetData()
	if string(data) != "test data" {
		t.Errorf("GetData() = %q, want %q", data, "test data")
	}

	// Verify it's a copy
	data[0] = 'X'
	if string(buffer.data) == "Xest data" {
		t.Error("GetData() should return a copy, not the original")
	}
}

func TestScrollbackBufferGetRecentLines(t *testing.T) {
	buffer := NewScrollbackBuffer(1000)

	t.Run("empty buffer", func(t *testing.T) {
		lines := buffer.GetRecentLines(5)
		if lines != nil {
			t.Errorf("expected nil, got %q", lines)
		}
	})

	t.Run("less lines than requested", func(t *testing.T) {
		buffer.data = []byte("line1\nline2\nline3")
		lines := buffer.GetRecentLines(10)
		if string(lines) != "line1\nline2\nline3" {
			t.Errorf("got %q, want all lines", lines)
		}
	})

	t.Run("more lines than requested", func(t *testing.T) {
		buffer.data = []byte("line1\nline2\nline3\nline4\nline5")
		lines := buffer.GetRecentLines(2)
		// Should get last 2 lines
		if !bytes.Contains(lines, []byte("line5")) {
			t.Errorf("got %q, expected to contain line5", lines)
		}
	})
}

func TestScrollbackBufferClear(t *testing.T) {
	buffer := NewScrollbackBuffer(100)
	buffer.Write([]byte("test data"))

	buffer.Clear()
	if len(buffer.data) != 0 {
		t.Errorf("data length after clear = %d, want 0", len(buffer.data))
	}
}

func TestScrollbackBufferConcurrency(t *testing.T) {
	buffer := NewScrollbackBuffer(10000)
	done := make(chan bool, 4)

	// Writer 1
	go func() {
		for i := 0; i < 100; i++ {
			buffer.Write([]byte("writer1 data\n"))
		}
		done <- true
	}()

	// Writer 2
	go func() {
		for i := 0; i < 100; i++ {
			buffer.Write([]byte("writer2 data\n"))
		}
		done <- true
	}()

	// Reader 1
	go func() {
		for i := 0; i < 100; i++ {
			_ = buffer.GetData()
		}
		done <- true
	}()

	// Reader 2
	go func() {
		for i := 0; i < 100; i++ {
			_ = buffer.GetRecentLines(10)
		}
		done <- true
	}()

	for i := 0; i < 4; i++ {
		<-done
	}
}

func BenchmarkScrollbackBufferWrite(b *testing.B) {
	buffer := NewScrollbackBuffer(DefaultScrollbackSize)
	data := []byte("benchmark test data line\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Write(data)
	}
}

func BenchmarkScrollbackBufferGetData(b *testing.B) {
	buffer := NewScrollbackBuffer(DefaultScrollbackSize)
	buffer.Write(make([]byte, DefaultScrollbackSize/2))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buffer.GetData()
	}
}
