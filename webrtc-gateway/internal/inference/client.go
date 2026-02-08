package inference

import (
	"context"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	whatsv1 "github.com/RenatoCabral2022/WhatsWebService/gen/go/whats/v1"
)

// Client provides gRPC connections to the ASR and TTS inference services.
type Client struct {
	asrConn   *grpc.ClientConn
	ttsConn   *grpc.ClientConn
	asrClient whatsv1.AsrServiceClient
	ttsClient whatsv1.TtsServiceClient
}

// NewClient creates a new inference client with gRPC connections to ASR and TTS.
func NewClient(asrAddr, ttsAddr string) (*Client, error) {
	asrConn, err := grpc.NewClient(asrAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10*1024*1024)),
	)
	if err != nil {
		return nil, err
	}

	ttsConn, err := grpc.NewClient(ttsAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		asrConn.Close()
		return nil, err
	}

	return &Client{
		asrConn:   asrConn,
		ttsConn:   ttsConn,
		asrClient: whatsv1.NewAsrServiceClient(asrConn),
		ttsClient: whatsv1.NewTtsServiceClient(ttsConn),
	}, nil
}

// Transcribe sends audio to the ASR service and returns the transcription.
func (c *Client) Transcribe(ctx context.Context, audio []byte, sessionID, actionID, languageHint, task string) (*whatsv1.TranscribeResponse, error) {
	return c.asrClient.Transcribe(ctx, &whatsv1.TranscribeRequest{
		Audio: audio,
		Format: &whatsv1.AudioFormat{
			SampleRate: 16000,
			Channels:   1,
			Encoding:   whatsv1.AudioEncoding_AUDIO_ENCODING_PCM_S16LE,
		},
		SessionId:    sessionID,
		ActionId:     actionID,
		LanguageHint: languageHint,
		Task:         task,
	})
}

// SynthesizeStream calls TTS and returns channels for audio chunks and errors.
// The chunks channel receives PCM s16le 16kHz mono byte slices.
// Both channels are closed when the stream ends.
func (c *Client) SynthesizeStream(ctx context.Context, text, sessionID, actionID, voice, language string, speed float32) (<-chan []byte, <-chan error) {
	chunks := make(chan []byte, 16)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		stream, err := c.ttsClient.Synthesize(ctx, &whatsv1.SynthesizeRequest{
			Text:      text,
			SessionId: sessionID,
			ActionId:  actionID,
			Voice:     voice,
			Speed:     speed,
			Language:  language,
			OutputFormat: &whatsv1.AudioFormat{
				SampleRate: 16000,
				Channels:   1,
				Encoding:   whatsv1.AudioEncoding_AUDIO_ENCODING_PCM_S16LE,
			},
		})
		if err != nil {
			errs <- err
			return
		}

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				errs <- err
				return
			}
			if resp.Chunk != nil && len(resp.Chunk.Data) > 0 {
				select {
				case chunks <- resp.Chunk.Data:
				case <-ctx.Done():
					return
				}
			}
			if resp.Chunk != nil && resp.Chunk.IsFinal {
				return
			}
		}
	}()

	return chunks, errs
}

// Close shuts down all gRPC connections.
func (c *Client) Close() {
	if c.asrConn != nil {
		c.asrConn.Close()
	}
	if c.ttsConn != nil {
		c.ttsConn.Close()
	}
}
