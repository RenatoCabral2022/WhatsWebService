package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"go.uber.org/zap"

	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/audio"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/datachannel"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/ringbuffer"
)

// Session holds all per-connection state.
type Session struct {
	ID         string
	RingBuffer *ringbuffer.RingBuffer

	mu         sync.Mutex
	pc         *webrtc.PeerConnection
	dc         *webrtc.DataChannel
	audioTrack *webrtc.TrackLocalStaticSample
	decoder    *audio.Decoder
	encoder    *audio.Encoder
	router     *datachannel.Router
	logger     *zap.Logger
	stopCh     chan struct{}
	stopped    bool

	activeAction string
	actionCancel context.CancelFunc

	lastSeqNum uint16
	seqNumInit bool
}

// New creates a new session with a ring buffer of the specified duration.
func New(id string, ringBufferSeconds int, logger *zap.Logger) *Session {
	return &Session{
		ID:         id,
		RingBuffer: ringbuffer.New(ringBufferSeconds),
		logger:     logger.With(zap.String("session", id)),
		stopCh:     make(chan struct{}),
	}
}

func (s *Session) SetPeerConnection(pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticSample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pc = pc
	s.audioTrack = track
}

func (s *Session) SetDataChannel(dc *webrtc.DataChannel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dc = dc
}

func (s *Session) SetCodecs(dec *audio.Decoder, enc *audio.Encoder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decoder = dec
	s.encoder = enc
}

func (s *Session) SetRouter(r *datachannel.Router) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.router = r
}

// TryStartAction attempts to claim the session for an action.
// If another action is running, it cancels it first (auto-cancel-and-replace).
// Returns a context that will be cancelled if the action is superseded or session stops.
func (s *Session) TryStartAction(actionID string) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel any existing action
	if s.actionCancel != nil {
		s.actionCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.activeAction = actionID
	s.actionCancel = cancel
	return ctx
}

// FinishAction clears the active action if it matches the given actionID.
func (s *Session) FinishAction(actionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeAction == actionID {
		s.activeAction = ""
		s.actionCancel = nil
	}
}

// PlayPCMStream reads 16kHz PCM s16le chunks from the channel, upsamples to 48kHz,
// encodes to Opus, and writes to the outbound WebRTC track at real-time pace.
func (s *Session) PlayPCMStream(ctx context.Context, chunks <-chan []byte) error {
	s.mu.Lock()
	enc := s.encoder
	track := s.audioTrack
	s.mu.Unlock()

	if enc == nil || track == nil {
		return fmt.Errorf("encoder or track not set")
	}

	frameDuration := time.Duration(audio.FrameDurationMs) * time.Millisecond
	// 320 samples at 16kHz = 20ms (one Opus frame after upsample to 960 at 48kHz)
	samplesPerFrame16k := audio.SamplesPerFrame / 3

	var residual []int16

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return fmt.Errorf("session stopped")
		case chunk, ok := <-chunks:
			if !ok {
				// Channel closed — drain residual if any
				if len(residual) > 0 {
					// Pad residual to full frame with zeros
					for len(residual) < samplesPerFrame16k {
						residual = append(residual, 0)
					}
					frame48k := audio.Upsample16to48(residual[:samplesPerFrame16k])
					opusData, err := enc.Encode(frame48k)
					if err == nil {
						track.WriteSample(media.Sample{
							Data:     opusData,
							Duration: frameDuration,
						})
					}
				}
				return nil
			}

			samples16k := audio.BytesToInt16(chunk)
			if len(residual) > 0 {
				samples16k = append(residual, samples16k...)
				residual = nil
			}

			// Process in frames of samplesPerFrame16k (320 samples = 20ms at 16kHz)
			for len(samples16k) >= samplesPerFrame16k {
				frame16k := samples16k[:samplesPerFrame16k]
				samples16k = samples16k[samplesPerFrame16k:]

				frame48k := audio.Upsample16to48(frame16k)
				opusData, err := enc.Encode(frame48k)
				if err != nil {
					s.logger.Warn("opus encode failed in stream", zap.Error(err))
					continue
				}

				if err := track.WriteSample(media.Sample{
					Data:     opusData,
					Duration: frameDuration,
				}); err != nil {
					return fmt.Errorf("write sample: %w", err)
				}

				time.Sleep(frameDuration)
			}

			// Save leftover samples
			if len(samples16k) > 0 {
				residual = make([]int16, len(samples16k))
				copy(residual, samples16k)
			}
		}
	}
}

// HandleInboundRTP decodes an Opus packet, downsamples 48k→16k, and writes to the ring buffer.
// Detects sequence number gaps and applies PLC for missing frames.
func (s *Session) HandleInboundRTP(seqNum uint16, opusData []byte) {
	s.mu.Lock()
	dec := s.decoder
	s.mu.Unlock()

	if dec == nil {
		return
	}

	// Detect gaps in RTP sequence numbers for PLC
	if s.seqNumInit {
		expected := s.lastSeqNum + 1
		if seqNum != expected {
			gap := int(seqNum - expected)
			if gap > 0 && gap < 100 {
				for i := 0; i < gap; i++ {
					plc, err := dec.DecodePLC(audio.OpusSampleRate * audio.FrameDurationMs / 1000)
					if err != nil {
						s.logger.Warn("PLC decode failed", zap.Error(err))
						continue
					}
					down := audio.Downsample48to16(plc)
					s.RingBuffer.Write(audio.Int16ToBytes(down))
				}
			}
		}
	}
	s.lastSeqNum = seqNum
	s.seqNumInit = true

	// Decode real Opus frame
	pcm48, err := dec.Decode(opusData)
	if err != nil {
		s.logger.Warn("opus decode failed", zap.Error(err))
		return
	}

	// Downsample 48kHz → 16kHz and write to ring buffer
	pcm16 := audio.Downsample48to16(pcm48)
	s.RingBuffer.Write(audio.Int16ToBytes(pcm16))
}

// PlayTestTone generates a sine wave, encodes it to Opus, and writes it to the outbound track.
// Blocks for the duration of the tone.
func (s *Session) PlayTestTone(durationSec float64) {
	s.mu.Lock()
	enc := s.encoder
	track := s.audioTrack
	s.mu.Unlock()

	if enc == nil || track == nil {
		s.logger.Warn("cannot play test tone: encoder or track not set")
		return
	}

	// Generate 16kHz sine wave and upsample to 48kHz for Opus encoding
	pcm16 := audio.GenerateSineWave(durationSec, audio.ToneFrequency)
	pcm48 := audio.Upsample16to48(pcm16)

	frameDuration := time.Duration(audio.FrameDurationMs) * time.Millisecond

	// Chunk into 960-sample (20ms) frames and encode/send
	for i := 0; i+audio.SamplesPerFrame <= len(pcm48); i += audio.SamplesPerFrame {
		select {
		case <-s.stopCh:
			return
		default:
		}

		frame := pcm48[i : i+audio.SamplesPerFrame]
		opusData, err := enc.Encode(frame)
		if err != nil {
			s.logger.Warn("opus encode failed", zap.Error(err))
			return
		}

		if err := track.WriteSample(media.Sample{
			Data:     opusData,
			Duration: frameDuration,
		}); err != nil {
			s.logger.Warn("write sample failed", zap.Error(err))
			return
		}

		time.Sleep(frameDuration)
	}
}

// SendDataChannelMessage sends a JSON message over the data channel.
func (s *Session) SendDataChannelMessage(msg interface{}) error {
	s.mu.Lock()
	dc := s.dc
	s.mu.Unlock()

	if dc == nil {
		return nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return dc.SendText(string(data))
}

// SetRemoteDescription applies the client's SDP answer to the PeerConnection.
func (s *Session) SetRemoteDescription(desc webrtc.SessionDescription) error {
	s.mu.Lock()
	pc := s.pc
	s.mu.Unlock()

	if pc == nil {
		return fmt.Errorf("peer connection not available")
	}
	return pc.SetRemoteDescription(desc)
}

// Stop closes the session. Idempotent.
func (s *Session) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return
	}
	s.stopped = true

	if s.actionCancel != nil {
		s.actionCancel()
		s.actionCancel = nil
	}

	close(s.stopCh)

	if s.pc != nil {
		s.pc.Close()
	}
	s.logger.Info("session stopped")
}

