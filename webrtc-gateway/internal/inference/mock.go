package inference

import (
	"context"
	"time"

	whatsv1 "github.com/RenatoCabral2022/WhatsWebService/gen/go/whats/v1"
)

// MockClient returns canned responses for testing.
type MockClient struct {
	TranscribeDelay time.Duration
	TranscribeText  string
	TTSChunkDelay   time.Duration
	TTSChunkCount   int
	TTSChunkSize    int // bytes per chunk (default 3200 = 100ms at 16kHz)
}

func (m *MockClient) Transcribe(ctx context.Context, audio []byte, sessionID, actionID, languageHint, task string) (*whatsv1.TranscribeResponse, error) {
	select {
	case <-time.After(m.TranscribeDelay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	text := m.TranscribeText
	if text == "" {
		text = "hello world"
	}
	return &whatsv1.TranscribeResponse{
		Text:     text,
		Language: "en",
	}, nil
}

func (m *MockClient) SynthesizeStream(ctx context.Context, text, sessionID, actionID, voice, language string, speed float32) (<-chan []byte, <-chan error) {
	count := m.TTSChunkCount
	if count == 0 {
		count = 10
	}
	chunkSize := m.TTSChunkSize
	if chunkSize == 0 {
		chunkSize = 3200
	}

	chunks := make(chan []byte, count)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)
		for i := 0; i < count; i++ {
			select {
			case <-time.After(m.TTSChunkDelay):
			case <-ctx.Done():
				return
			}
			select {
			case chunks <- make([]byte, chunkSize):
			case <-ctx.Done():
				return
			}
		}
	}()

	return chunks, errs
}

func (m *MockClient) Close() {}
