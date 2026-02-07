package ringbuffer

import "sync"

// BytesPerSecond is the number of bytes per second for PCM s16le, 16kHz, mono audio.
// 16000 samples/sec * 2 bytes/sample = 32000 bytes/sec.
const BytesPerSecond = 16000 * 2

// RingBuffer holds a fixed-duration circular buffer of PCM s16le, 16kHz, mono audio.
// It is safe for concurrent use from a single writer and single reader.
type RingBuffer struct {
	mu       sync.Mutex
	buf      []byte
	writePos int
	capacity int
	written  int // total bytes ever written (for tracking fill level)
}

// New creates a ring buffer that holds the specified number of seconds of audio.
func New(seconds int) *RingBuffer {
	cap := seconds * BytesPerSecond
	return &RingBuffer{
		buf:      make([]byte, cap),
		capacity: cap,
	}
}

// Write appends PCM data to the buffer, overwriting the oldest data when full.
func (rb *RingBuffer) Write(data []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for len(data) > 0 {
		n := copy(rb.buf[rb.writePos:], data)
		data = data[n:]
		rb.writePos = (rb.writePos + n) % rb.capacity
		rb.written += n
	}
}

// Snapshot returns a copy of the last N seconds of audio.
// If less data has been written than requested, only the available data is returned.
func (rb *RingBuffer) Snapshot(seconds int) []byte {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	requested := seconds * BytesPerSecond
	if requested > rb.capacity {
		requested = rb.capacity
	}

	available := rb.written
	if available > rb.capacity {
		available = rb.capacity
	}
	if requested > available {
		requested = available
	}

	if requested == 0 {
		return nil
	}

	out := make([]byte, requested)
	start := (rb.writePos - requested + rb.capacity) % rb.capacity

	if start+requested <= rb.capacity {
		copy(out, rb.buf[start:start+requested])
	} else {
		first := rb.capacity - start
		copy(out[:first], rb.buf[start:])
		copy(out[first:], rb.buf[:requested-first])
	}

	return out
}

// Available returns the number of seconds of audio currently stored.
func (rb *RingBuffer) Available() float64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	available := rb.written
	if available > rb.capacity {
		available = rb.capacity
	}
	return float64(available) / float64(BytesPerSecond)
}
