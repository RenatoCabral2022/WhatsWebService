package ingest

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/ringbuffer"
)

// chunkSize is 640 bytes = 20ms of 16kHz mono PCM s16le.
// Matches the WebRTC output frame duration for minimal burstiness.
const chunkSize = 640

// FFmpegURLSource ingests audio from a URL using ffmpeg, normalizing to
// PCM s16le 16kHz mono and writing to a ring buffer.
type FFmpegURLSource struct {
	url    string
	rb     *ringbuffer.RingBuffer
	maxDur time.Duration
	logger *zap.Logger

	mu        sync.Mutex
	state     string
	lastError string
	cancel    context.CancelFunc
	cmd       *exec.Cmd

	bytesRead atomic.Int64
}

// NewFFmpegURLSource creates a new ffmpeg-based URL ingest source.
func NewFFmpegURLSource(sourceURL string, rb *ringbuffer.RingBuffer,
	maxDurationSec int, logger *zap.Logger) *FFmpegURLSource {

	return &FFmpegURLSource{
		url:    sourceURL,
		rb:     rb,
		maxDur: time.Duration(maxDurationSec) * time.Second,
		logger: logger.With(zap.String("ingestURL", sourceURL)),
		state:  StateStopped,
	}
}

// Start begins ingesting audio. Blocks until the source ends, ctx is cancelled, or Stop is called.
func (f *FFmpegURLSource) Start(ctx context.Context) error {
	f.mu.Lock()
	if f.state == StateRunning || f.state == StateStarting {
		f.mu.Unlock()
		return fmt.Errorf("ingest already running")
	}
	f.state = StateStarting
	f.lastError = ""
	f.bytesRead.Store(0)

	ingestCtx, cancel := context.WithCancel(ctx)
	if f.maxDur > 0 {
		ingestCtx, cancel = context.WithTimeout(ctx, f.maxDur)
	}
	f.cancel = cancel
	f.mu.Unlock()

	defer cancel()

	args := []string{
		"-nostdin",
		"-hide_banner", "-loglevel", "error",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-i", f.url,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-f", "s16le",
		"pipe:1",
	}

	cmd := exec.CommandContext(ingestCtx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		f.setError(fmt.Sprintf("stdout pipe: %v", err))
		return err
	}

	if err := cmd.Start(); err != nil {
		f.setError(fmt.Sprintf("ffmpeg start: %v", err))
		return err
	}

	f.mu.Lock()
	f.cmd = cmd
	f.state = StateRunning
	f.mu.Unlock()

	f.logger.Info("ingest started")

	// Read PCM from stdout and write to ring buffer
	readErr := f.readLoop(ingestCtx, stdout)

	// Wait for ffmpeg to exit
	waitErr := cmd.Wait()

	f.mu.Lock()
	defer f.mu.Unlock()

	if ingestCtx.Err() != nil {
		f.state = StateStopped
		f.logger.Info("ingest stopped", zap.Int64("bytesRead", f.bytesRead.Load()))
		return nil
	}

	if readErr != nil || waitErr != nil {
		errMsg := ""
		if readErr != nil {
			errMsg = readErr.Error()
		} else if waitErr != nil {
			errMsg = waitErr.Error()
		}
		f.state = StateError
		f.lastError = errMsg
		f.logger.Warn("ingest error", zap.String("error", errMsg))
		return fmt.Errorf("ingest failed: %s", errMsg)
	}

	// Normal EOF (file ended)
	f.state = StateStopped
	f.logger.Info("ingest completed (source ended)",
		zap.Int64("bytesRead", f.bytesRead.Load()))
	return nil
}

func (f *FFmpegURLSource) readLoop(ctx context.Context, r io.Reader) error {
	buf := make([]byte, chunkSize)
	capSec := f.rb.CapacitySeconds()
	lastLog := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, err := r.Read(buf)
		if n > 0 {
			f.rb.Write(buf[:n])
			f.bytesRead.Add(int64(n))

			// Periodic progress log every 5 seconds
			if time.Since(lastLog) >= 5*time.Second {
				f.logger.Info("ingest progress",
					zap.Float64("bufferedSec", f.rb.Available()),
					zap.Int64("bytesRead", f.bytesRead.Load()))
				lastLog = time.Now()
			}

			// Fill-then-realtime throttling: once buffer is full,
			// pace writes at ~1x realtime to avoid wasting CPU on file URLs.
			if f.rb.Available() >= capSec {
				// 640 bytes = 20ms of audio at 16kHz s16le mono
				sleepMs := float64(n) / float64(ringbuffer.BytesPerSecond) * 1000
				time.Sleep(time.Duration(sleepMs) * time.Millisecond)
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// Stop terminates the ingest. Idempotent.
func (f *FFmpegURLSource) Stop() {
	f.mu.Lock()
	cancel := f.cancel
	f.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Status returns a snapshot of current ingest state.
func (f *FFmpegURLSource) Status() Status {
	f.mu.Lock()
	state := f.state
	lastErr := f.lastError
	f.mu.Unlock()

	return Status{
		State:           state,
		SourceURL:       f.url,
		SecondsBuffered: f.rb.Available(),
		BytesRead:       f.bytesRead.Load(),
		LastError:       lastErr,
	}
}

func (f *FFmpegURLSource) setError(msg string) {
	f.mu.Lock()
	f.state = StateError
	f.lastError = msg
	f.mu.Unlock()
}
