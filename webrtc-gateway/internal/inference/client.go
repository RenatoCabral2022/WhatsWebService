package inference

// Client provides gRPC connections to the ASR and TTS inference services.
// TODO: implement with generated proto stubs in Phase 2.
type Client struct {
	ASRAddr string
	TTSAddr string
}

// NewClient creates a new inference client.
func NewClient(asrAddr, ttsAddr string) *Client {
	return &Client{
		ASRAddr: asrAddr,
		TTSAddr: ttsAddr,
	}
}
