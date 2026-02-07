package audio

// Decoder converts Opus audio frames to PCM s16le, 16kHz, mono.
// TODO: implement using pion/opus or a CGo Opus binding in Phase 2.
type Decoder struct{}

// Decode converts an Opus frame to PCM s16le bytes.
func (d *Decoder) Decode(opusFrame []byte) ([]byte, error) {
	// TODO: implement Opus decoding
	return nil, nil
}
