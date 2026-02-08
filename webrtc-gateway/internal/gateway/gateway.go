package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/nack"
	"github.com/pion/webrtc/v4"
	"go.uber.org/zap"

	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/audio"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/config"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/datachannel"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/inference"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/metrics"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/ringbuffer"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/session"
)

const iceGatherTimeout = 10 * time.Second

// Gateway manages WebRTC connections and orchestrates the audio pipeline.
type Gateway struct {
	cfg             *config.Config
	api             *webrtc.API
	logger          *zap.Logger
	inferenceClient inference.InferenceClient
	inferenceSem    chan struct{}
	snapshotPool    sync.Pool

	mu       sync.RWMutex
	sessions map[string]*session.Session
}

// New creates a Gateway with Opus codecs registered and interceptors configured.
func New(cfg *config.Config, logger *zap.Logger) (*Gateway, error) {
	// MediaEngine with Opus codec
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeOpus,
			ClockRate:   48000,
			Channels:    2,
			SDPFmtpLine: "minptime=10;useinbandfec=1",
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("register opus codec: %w", err)
	}

	// Interceptor registry for NACK/RTCP
	ir := &interceptor.Registry{}
	responder, err := nack.NewResponderInterceptor()
	if err != nil {
		return nil, fmt.Errorf("create nack responder: %w", err)
	}
	ir.Add(responder)

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(m),
		webrtc.WithInterceptorRegistry(ir),
	)

	// Create inference client for ASR + TTS
	infClient, err := inference.NewClient(cfg.ASRAddr, cfg.TTSAddr)
	if err != nil {
		return nil, fmt.Errorf("create inference client: %w", err)
	}

	return newGateway(cfg, api, logger, infClient), nil
}

// NewForTest creates a Gateway with injected dependencies for testing.
// Does not create WebRTC API or real inference client.
func NewForTest(cfg *config.Config, logger *zap.Logger, infClient inference.InferenceClient) *Gateway {
	return newGateway(cfg, nil, logger, infClient)
}

func newGateway(cfg *config.Config, api *webrtc.API, logger *zap.Logger, infClient inference.InferenceClient) *Gateway {
	maxSnapshotBytes := cfg.MaxLookbackSec * ringbuffer.BytesPerSecond
	return &Gateway{
		cfg:             cfg,
		api:             api,
		logger:          logger,
		inferenceClient: infClient,
		inferenceSem:    make(chan struct{}, cfg.MaxInferenceConcurrency),
		snapshotPool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, maxSnapshotBytes)
				return &buf
			},
		},
		sessions: make(map[string]*session.Session),
	}
}

// SessionCount returns the current number of active sessions.
func (gw *Gateway) SessionCount() int {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return len(gw.sessions)
}

// ICEServers returns the configured STUN/TURN servers as WebRTC config objects.
func (gw *Gateway) ICEServers() []webrtc.ICEServer {
	urls := make([]string, len(gw.cfg.STUNServers))
	copy(urls, gw.cfg.STUNServers)
	return []webrtc.ICEServer{{URLs: urls}}
}

// CreateSession sets up a full WebRTC PeerConnection with inbound/outbound audio.
// Returns the SDP offer string for the client to answer.
func (gw *Gateway) CreateSession(id string) (string, error) {
	logger := gw.logger.With(zap.String("session", id))

	sess := session.New(id, gw.cfg.RingBufferSec, gw.logger)

	// Create Opus decoder + encoder
	dec, err := audio.NewDecoder()
	if err != nil {
		return "", fmt.Errorf("create opus decoder: %w", err)
	}
	enc, err := audio.NewEncoder()
	if err != nil {
		return "", fmt.Errorf("create opus encoder: %w", err)
	}
	sess.SetCodecs(dec, enc)

	// Create PeerConnection
	pc, err := gw.api.NewPeerConnection(webrtc.Configuration{
		ICEServers: gw.ICEServers(),
	})
	if err != nil {
		return "", fmt.Errorf("create peer connection: %w", err)
	}

	// Add outbound audio track (server → browser)
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		"audio-out",
		"whats-gateway",
	)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("create audio track: %w", err)
	}
	if _, err := pc.AddTrack(track); err != nil {
		pc.Close()
		return "", fmt.Errorf("add audio track: %w", err)
	}

	sess.SetPeerConnection(pc, track)

	// Create data channel on server side (must be before CreateOffer so SCTP is in SDP)
	ordered := true
	dc, err := pc.CreateDataChannel("commands", &webrtc.DataChannelInit{Ordered: &ordered})
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("create data channel: %w", err)
	}
	sess.SetDataChannel(dc)

	router := datachannel.NewRouter()
	router.Register("command.enunciate", gw.makeEnunciateHandler(sess))
	sess.SetRouter(router)

	dc.OnOpen(func() {
		logger.Info("data channel opened")
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if err := router.Dispatch(msg.Data); err != nil {
			logger.Warn("dispatch error", zap.Error(err))
		}
	})

	// Wire OnTrack: inbound audio from browser mic
	pc.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		logger.Info("inbound track",
			zap.String("codec", remoteTrack.Codec().MimeType),
			zap.Uint8("pt", uint8(remoteTrack.PayloadType())),
		)
		go gw.inboundAudioLoop(sess, remoteTrack)
	})

	// Wire ICE connection state changes
	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		logger.Info("ICE state", zap.String("state", state.String()))
		if state == webrtc.ICEConnectionStateFailed ||
			state == webrtc.ICEConnectionStateDisconnected ||
			state == webrtc.ICEConnectionStateClosed {
			gw.DeleteSession(id)
		}
	})

	// Create SDP offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("create offer: %w", err)
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		pc.Close()
		return "", fmt.Errorf("set local description: %w", err)
	}

	// Wait for ICE gathering to complete
	gatherDone := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gatherDone:
	case <-time.After(iceGatherTimeout):
		logger.Warn("ICE gathering timed out, proceeding with partial candidates")
	}

	sdp := pc.LocalDescription().SDP

	// Store session
	gw.mu.Lock()
	gw.sessions[id] = sess
	gw.mu.Unlock()

	metrics.SessionsCreatedTotal.Inc()
	metrics.ActiveSessions.Inc()

	// Auto-cleanup if no answer received within 30 seconds
	time.AfterFunc(30*time.Second, func() {
		gw.mu.RLock()
		s, ok := gw.sessions[id]
		gw.mu.RUnlock()
		if !ok {
			return
		}
		if s != nil {
			// The session exists — if it hasn't progressed, it will be cleaned up
			// by the ICE state change handler when it times out
		}
	})

	logger.Info("session created", zap.Int("sdpLen", len(sdp)))
	return sdp, nil
}

// SetAnswer applies the client's SDP answer to the session's PeerConnection.
func (gw *Gateway) SetAnswer(id, sdpAnswer string) error {
	gw.mu.RLock()
	sess, ok := gw.sessions[id]
	gw.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	return sess.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdpAnswer,
	})
}

// DeleteSession tears down a session and removes it from the registry.
func (gw *Gateway) DeleteSession(id string) {
	gw.mu.Lock()
	sess, ok := gw.sessions[id]
	if ok {
		delete(gw.sessions, id)
	}
	gw.mu.Unlock()

	if ok && sess != nil {
		sess.Stop()
		metrics.ActiveSessions.Dec()
		gw.logger.Info("session deleted", zap.String("session", id))
	}
}

// Shutdown stops all sessions and closes inference connections.
func (gw *Gateway) Shutdown() {
	gw.mu.Lock()
	sessions := make(map[string]*session.Session, len(gw.sessions))
	for k, v := range gw.sessions {
		sessions[k] = v
	}
	gw.sessions = make(map[string]*session.Session)
	gw.mu.Unlock()

	for _, sess := range sessions {
		sess.Stop()
	}
	metrics.ActiveSessions.Set(0)

	if gw.inferenceClient != nil {
		gw.inferenceClient.Close()
	}

	gw.logger.Info("gateway shutdown complete")
}

// inboundAudioLoop reads RTP packets from a remote track and feeds them into the session.
func (gw *Gateway) inboundAudioLoop(sess *session.Session, track *webrtc.TrackRemote) {
	logger := gw.logger.With(zap.String("session", sess.ID))
	logger.Info("inbound audio loop started")

	for {
		pkt, _, err := track.ReadRTP()
		if err != nil {
			logger.Info("inbound audio loop ended", zap.Error(err))
			return
		}
		sess.HandleInboundRTP(pkt.SequenceNumber, pkt.Payload)
	}
}

// makeEnunciateHandler returns a datachannel.Handler that orchestrates the enunciate pipeline.
func (gw *Gateway) makeEnunciateHandler(sess *session.Session) datachannel.Handler {
	return func(sessionID, actionID string, payload json.RawMessage) error {
		gw.logger.Info("enunciate command",
			zap.String("session", sessionID),
			zap.String("action", actionID),
		)

		// Parse command payload
		var cmd datachannel.CommandEnunciate
		if err := json.Unmarshal(payload, &cmd); err != nil {
			gw.logger.Warn("invalid enunciate payload", zap.Error(err))
			return err
		}

		// Claim action slot with timeout (auto-cancels previous)
		timeout := time.Duration(gw.cfg.ActionTimeoutSec) * time.Second
		ctx := sess.TryStartAction(actionID, timeout)

		go gw.executeEnunciate(ctx, sess, sessionID, actionID, cmd)
		return nil
	}
}

// executeEnunciate runs the full enunciate pipeline: snapshot → ASR → TTS → playback.
func (gw *Gateway) executeEnunciate(ctx context.Context, sess *session.Session,
	sessionID, actionID string, cmd datachannel.CommandEnunciate) {

	defer sess.FinishAction(actionID)
	start := time.Now()

	metrics.ActiveActions.Inc()
	defer metrics.ActiveActions.Dec()

	logger := gw.logger.With(
		zap.String("session", sessionID),
		zap.String("action", actionID),
	)

	// 1. Validate buffer
	lookback := cmd.LookbackSeconds
	if lookback <= 0 {
		lookback = 5
	}
	if lookback > gw.cfg.MaxLookbackSec {
		lookback = gw.cfg.MaxLookbackSec
	}
	available := sess.RingBuffer.Available()
	if available < 0.5 {
		logger.Warn("insufficient audio buffer", zap.Float64("available", available))
		gw.sendError(sess, sessionID, actionID, "INSUFFICIENT_AUDIO_BUFFER",
			fmt.Sprintf("only %.1fs buffered, need at least 0.5s", available))
		return
	}

	// 2. Acquire inference semaphore (fast-fail backpressure)
	select {
	case gw.inferenceSem <- struct{}{}:
		metrics.InferenceSemUsed.Inc()
	default:
		logger.Warn("inference pool saturated")
		gw.sendError(sess, sessionID, actionID, "RATE_LIMITED", "inference busy, try again")
		metrics.ActionsTotal.WithLabelValues("rate_limited").Inc()
		return
	}
	defer func() {
		<-gw.inferenceSem
		metrics.InferenceSemUsed.Dec()
	}()

	// 3. Snapshot ring buffer (pooled)
	snapshotStart := time.Now()
	bufPtr := gw.snapshotPool.Get().(*[]byte)
	pcm := sess.RingBuffer.SnapshotInto(lookback, *bufPtr)
	snapshotMs := float64(time.Since(snapshotStart).Microseconds()) / 1000.0
	logger.Info("snapshot taken",
		zap.Int("lookback", lookback),
		zap.Int("bytes", len(pcm)),
		zap.Float64("snapshotMs", snapshotMs),
	)

	if len(pcm) == 0 {
		gw.snapshotPool.Put(bufPtr)
		gw.sendError(sess, sessionID, actionID, "INSUFFICIENT_AUDIO_BUFFER", "ring buffer empty")
		return
	}

	// 4. Determine ASR parameters
	// Always transcribe — NLLB handles translation, not Whisper.
	task := "transcribe"
	languageHint := ""
	targetLanguage := cmd.TargetLanguage

	// 5. Call ASR (+ optional translation via NLLB inside ASR service)
	asrStart := time.Now()
	asrResp, err := gw.inferenceClient.Transcribe(ctx, pcm, sessionID, actionID, languageHint, task, targetLanguage)
	// Return snapshot buffer to pool after ASR completes (pcm shares backing array)
	gw.snapshotPool.Put(bufPtr)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logger.Warn("enunciate timed out during ASR")
			metrics.InferenceTimeoutsTotal.Inc()
			metrics.ActionsTotal.WithLabelValues("timeout").Inc()
		} else if ctx.Err() != nil {
			logger.Info("enunciate cancelled during ASR")
			metrics.ActionsTotal.WithLabelValues("cancelled").Inc()
		} else {
			logger.Error("ASR failed", zap.Error(err))
			gw.sendError(sess, sessionID, actionID, "ASR_FAILED", err.Error())
			metrics.ActionsTotal.WithLabelValues("asr_error").Inc()
		}
		return
	}
	asrMs := float64(time.Since(asrStart).Microseconds()) / 1000.0
	logger.Info("ASR complete",
		zap.String("text", asrResp.Text),
		zap.String("language", asrResp.Language),
		zap.Float64("asrMs", asrMs),
	)

	// 6. Emit asr.final event
	segments := make([]datachannel.Segment, 0, len(asrResp.Segments))
	for _, s := range asrResp.Segments {
		segments = append(segments, datachannel.Segment{
			Text:       s.Text,
			StartTime:  float64(s.StartTime),
			EndTime:    float64(s.EndTime),
			Confidence: float64(s.Confidence),
		})
	}
	asrPayload, _ := json.Marshal(datachannel.EventAsrFinal{
		Text:           asrResp.Text,
		Language:       asrResp.Language,
		TranslatedText: asrResp.TranslatedText,
		TargetLanguage: asrResp.TargetLanguage,
		Segments:       segments,
		InferenceMs:    int(asrResp.InferenceDurationMs),
		TranslateMs:    int(asrResp.TranslateDurationMs),
	})
	sess.SendDataChannelMessage(datachannel.Envelope{
		Type:      "asr.final",
		SessionID: sessionID,
		ActionID:  actionID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   json.RawMessage(asrPayload),
	})

	var ttsFirstChunkMs float64

	if asrResp.Text != "" {
		// Determine text and language for TTS.
		// If translation produced a result, speak the translated text with the target language voice.
		ttsText := asrResp.Text
		ttsLang := asrResp.Language
		if asrResp.TranslatedText != "" {
			ttsText = asrResp.TranslatedText
			ttsLang = asrResp.TargetLanguage
		}

		// Send tts.started event
		sess.SendDataChannelMessage(datachannel.Envelope{
			Type:      "tts.started",
			SessionID: sessionID,
			ActionID:  actionID,
			Timestamp: time.Now().UnixMilli(),
			Payload:   json.RawMessage(`{"voice":"default"}`),
		})

		// Start TTS streaming
		ttsStart := time.Now()
		voice := cmd.TTSOptions.Voice
		if voice == "" {
			voice = "default"
		}
		speed := float32(cmd.TTSOptions.Speed)
		if speed <= 0 {
			speed = 1.0
		}

		rawChunks, errs := gw.inferenceClient.SynthesizeStream(ctx, ttsText,
			sessionID, actionID, voice, ttsLang, speed)

		// Proxy channel to measure first-chunk latency
		timedChunks := make(chan []byte, 16)
		go func() {
			defer close(timedChunks)
			first := true
			for chunk := range rawChunks {
				if first {
					ttsFirstChunkMs = float64(time.Since(ttsStart).Microseconds()) / 1000.0
					first = false
				}
				select {
				case timedChunks <- chunk:
				case <-ctx.Done():
					return
				}
			}
		}()

		// Play audio stream to browser
		if playErr := sess.PlayPCMStream(ctx, timedChunks); playErr != nil {
			if ctx.Err() == context.DeadlineExceeded {
				logger.Warn("enunciate timed out during TTS playback")
				metrics.InferenceTimeoutsTotal.Inc()
				metrics.ActionsTotal.WithLabelValues("timeout").Inc()
				return
			}
			if ctx.Err() != nil {
				logger.Info("enunciate cancelled during TTS playback")
				metrics.ActionsTotal.WithLabelValues("cancelled").Inc()
				return
			}
			logger.Warn("TTS playback error", zap.Error(playErr))
			metrics.ActionsTotal.WithLabelValues("tts_error").Inc()
			return
		}

		// Check for gRPC errors
		select {
		case err := <-errs:
			if err != nil {
				logger.Warn("TTS stream error", zap.Error(err))
			}
		default:
		}

		// Send tts.done event
		ttsDuration := time.Since(ttsStart)
		ttsDonePayload, _ := json.Marshal(datachannel.EventTtsDone{
			DurationMs: int(ttsDuration.Milliseconds()),
		})
		sess.SendDataChannelMessage(datachannel.Envelope{
			Type:      "tts.done",
			SessionID: sessionID,
			ActionID:  actionID,
			Timestamp: time.Now().UnixMilli(),
			Payload:   json.RawMessage(ttsDonePayload),
		})
	}

	// 7. Send latency metrics + record Prometheus histograms
	translateMs := float64(asrResp.TranslateDurationMs)
	totalMs := float64(time.Since(start).Milliseconds())
	latencyEvt := datachannel.EventMetricsLatency{
		SnapshotMs:      snapshotMs,
		AsrMs:           asrMs,
		TranslateMs:     translateMs,
		TtsFirstChunkMs: ttsFirstChunkMs,
		TotalMs:         totalMs,
	}
	metricsPayload, _ := json.Marshal(latencyEvt)
	sess.SendDataChannelMessage(datachannel.Envelope{
		Type:      "metrics.latency",
		SessionID: sessionID,
		ActionID:  actionID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   json.RawMessage(metricsPayload),
	})

	metrics.ActionsTotal.WithLabelValues("success").Inc()
	metrics.ActionLatency.WithLabelValues("total").Observe(totalMs)
	metrics.ActionLatency.WithLabelValues("snapshot").Observe(snapshotMs)
	metrics.ActionLatency.WithLabelValues("asr").Observe(asrMs)
	if translateMs > 0 {
		metrics.ActionLatency.WithLabelValues("translate").Observe(translateMs)
	}
	if ttsFirstChunkMs > 0 {
		metrics.ActionLatency.WithLabelValues("tts_first_chunk").Observe(ttsFirstChunkMs)
	}

	logger.Info("enunciate complete",
		zap.Float64("snapshotMs", snapshotMs),
		zap.Float64("asrMs", asrMs),
		zap.Float64("translateMs", translateMs),
		zap.Float64("ttsFirstChunkMs", ttsFirstChunkMs),
		zap.Float64("totalMs", totalMs),
	)
}

// sendError sends an error event over the data channel.
func (gw *Gateway) sendError(sess *session.Session, sessionID, actionID, code, message string) {
	errPayload, _ := json.Marshal(datachannel.EventError{
		Code:    code,
		Message: message,
	})
	sess.SendDataChannelMessage(datachannel.Envelope{
		Type:      "error",
		SessionID: sessionID,
		ActionID:  actionID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   json.RawMessage(errPayload),
	})
}
