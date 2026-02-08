package audio

import "github.com/hraban/opus"

const (
	OpusSampleRate = 48000
	OpusChannels   = 1
	MaxFrameSize   = 5760 // 120ms at 48kHz
)

// Decoder wraps hraban/opus to decode Opus frames to 48kHz int16 PCM.
// Not thread-safe — use one per session.
type Decoder struct {
	dec *opus.Decoder
}

func NewDecoder() (*Decoder, error) {
	dec, err := opus.NewDecoder(OpusSampleRate, OpusChannels)
	if err != nil {
		return nil, err
	}
	return &Decoder{dec: dec}, nil
}

// Decode converts a single Opus frame to 48kHz mono int16 samples.
func (d *Decoder) Decode(opusData []byte) ([]int16, error) {
	pcm := make([]int16, MaxFrameSize)
	n, err := d.dec.Decode(opusData, pcm)
	if err != nil {
		return nil, err
	}
	return pcm[:n], nil
}

// DecodePLC performs packet loss concealment for a missing frame.
func (d *Decoder) DecodePLC(expectedSamples int) ([]int16, error) {
	pcm := make([]int16, expectedSamples)
	n, err := d.dec.Decode(nil, pcm)
	if err != nil {
		return nil, err
	}
	return pcm[:n], nil
}
