package session

import "github.com/RenatoCabral2022/WHATS-SERVICE/webrtc-gateway/internal/ringbuffer"

// Session holds per-connection state including the audio ring buffer.
type Session struct {
	ID         string
	RingBuffer *ringbuffer.RingBuffer
	// TODO: add PeerConnection, DataChannel, gRPC client refs
}

// New creates a new session with a ring buffer of the specified duration.
func New(id string, ringBufferSeconds int) *Session {
	return &Session{
		ID:         id,
		RingBuffer: ringbuffer.New(ringBufferSeconds),
	}
}
