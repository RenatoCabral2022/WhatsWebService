package audio

// Encoder converts PCM s16le, 16kHz, mono audio to Opus for WebRTC output.
// TODO: implement using pion/opus or a CGo Opus binding in Phase 2.
type Encoder struct{}

// Encode converts PCM s16le bytes to an Opus frame.
func (e *Encoder) Encode(pcm []byte) ([]byte, error) {
	// TODO: implement Opus encoding
	return nil, nil
}
