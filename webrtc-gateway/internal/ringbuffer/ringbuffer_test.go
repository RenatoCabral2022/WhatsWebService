package ringbuffer

import (
	"bytes"
	"testing"
)

func TestNewCapacity(t *testing.T) {
	rb := New(5)
	if rb.capacity != 5*BytesPerSecond {
		t.Errorf("expected capacity %d, got %d", 5*BytesPerSecond, rb.capacity)
	}
}

func TestSnapshotEmpty(t *testing.T) {
	rb := New(5)
	snap := rb.Snapshot(1)
	if snap != nil {
		t.Errorf("expected nil snapshot from empty buffer, got %d bytes", len(snap))
	}
}

func TestWriteAndSnapshotExact(t *testing.T) {
	rb := New(1) // 1 second = 32000 bytes
	data := make([]byte, BytesPerSecond)
	for i := range data {
		data[i] = byte(i % 256)
	}
	rb.Write(data)

	snap := rb.Snapshot(1)
	if len(snap) != BytesPerSecond {
		t.Fatalf("expected snapshot len %d, got %d", BytesPerSecond, len(snap))
	}
	if !bytes.Equal(snap, data) {
		t.Error("snapshot data does not match written data")
	}
}

func TestSnapshotPartialFill(t *testing.T) {
	rb := New(5)
	data := make([]byte, BytesPerSecond) // write 1 second into a 5-second buffer
	rb.Write(data)

	snap := rb.Snapshot(3) // request 3 seconds but only 1 is available
	if len(snap) != BytesPerSecond {
		t.Errorf("expected %d bytes (1 second), got %d", BytesPerSecond, len(snap))
	}
}

func TestWrapAround(t *testing.T) {
	rb := New(1) // 1 second capacity

	// Write 1.5 seconds of data — first 0.5s should be overwritten
	first := make([]byte, BytesPerSecond/2)
	for i := range first {
		first[i] = 0xAA
	}
	second := make([]byte, BytesPerSecond)
	for i := range second {
		second[i] = 0xBB
	}

	rb.Write(first)
	rb.Write(second)

	snap := rb.Snapshot(1)
	if len(snap) != BytesPerSecond {
		t.Fatalf("expected %d bytes, got %d", BytesPerSecond, len(snap))
	}

	// The last BytesPerSecond bytes written should be all 0xBB
	for i, b := range snap {
		if b != 0xBB {
			t.Errorf("byte %d: expected 0xBB, got 0x%02X", i, b)
			break
		}
	}
}

func TestAvailable(t *testing.T) {
	rb := New(5)
	if rb.Available() != 0 {
		t.Errorf("expected 0 available, got %f", rb.Available())
	}

	rb.Write(make([]byte, BytesPerSecond*2))
	if rb.Available() != 2.0 {
		t.Errorf("expected 2.0 available, got %f", rb.Available())
	}

	// Write more than capacity
	rb.Write(make([]byte, BytesPerSecond*10))
	if rb.Available() != 5.0 {
		t.Errorf("expected 5.0 available (capped), got %f", rb.Available())
	}
}
