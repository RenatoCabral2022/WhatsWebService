package audio

import "sync"

// InboundFrameBuffers holds pre-allocated buffers for the RTP decode→downsample→bytes pipeline.
// Used via sync.Pool to avoid per-packet allocations in the hot path.
type InboundFrameBuffers struct {
	DecodeBuf     []int16 // cap: MaxFrameSize (5760)
	DownsampleBuf []int16 // cap: MaxFrameSize/3 (1920)
	BytesBuf      []byte  // cap: MaxFrameSize/3*2 (3840)
}

var inboundPool = sync.Pool{
	New: func() interface{} {
		return &InboundFrameBuffers{
			DecodeBuf:     make([]int16, MaxFrameSize),
			DownsampleBuf: make([]int16, MaxFrameSize/3),
			BytesBuf:      make([]byte, MaxFrameSize/3*2),
		}
	},
}

// AcquireInboundBuffers gets a set of buffers from the pool.
func AcquireInboundBuffers() *InboundFrameBuffers {
	return inboundPool.Get().(*InboundFrameBuffers)
}

// ReleaseInboundBuffers returns buffers to the pool.
func ReleaseInboundBuffers(b *InboundFrameBuffers) {
	inboundPool.Put(b)
}
