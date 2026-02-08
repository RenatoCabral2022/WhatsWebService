package audio

import "github.com/hraban/opus"

const (
	FrameDurationMs = 20
	SamplesPerFrame = OpusSampleRate * FrameDurationMs / 1000 // 960
	EncoderBitrate  = 32000
)

// Encoder wraps hraban/opus to encode 48kHz int16 PCM to Opus.
// Not thread-safe — use one per session.
type Encoder struct {
	enc *opus.Encoder
}

func NewEncoder() (*Encoder, error) {
	enc, err := opus.NewEncoder(OpusSampleRate, OpusChannels, opus.AppVoIP)
	if err != nil {
		return nil, err
	}
	if err := enc.SetBitrate(EncoderBitrate); err != nil {
		return nil, err
	}
	return &Encoder{enc: enc}, nil
}

// Encode converts a 960-sample (20ms at 48kHz) int16 frame to Opus bytes.
func (e *Encoder) Encode(pcm []int16) ([]byte, error) {
	buf := make([]byte, 1024)
	n, err := e.enc.Encode(pcm, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// EncodeInto encodes into a caller-provided buffer, avoiding allocation.
// Returns the used portion of buf.
func (e *Encoder) EncodeInto(pcm []int16, buf []byte) ([]byte, error) {
	n, err := e.enc.Encode(pcm, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}
