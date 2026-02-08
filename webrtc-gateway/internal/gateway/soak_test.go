//go:build soak

package gateway_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/config"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/inference"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/session"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/testutil"

	_ "github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/gateway"
	_ "github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/metrics"
)

const (
	soakDuration      = 2 * time.Minute
	soakSessions      = 5
	enunciateInterval = 5 * time.Second
)

func TestSoakStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	logger, _ := zap.NewDevelopment()

	cfg := &config.Config{
		RingBufferSec:           30,
		MaxSessions:             soakSessions + 5,
		MaxLookbackSec:          10,
		ActionTimeoutSec:        30,
		MaxInferenceConcurrency: 4,
	}

	mockClient := &inference.MockClient{
		TranscribeDelay: 50 * time.Millisecond,
		TranscribeText:  "hello world test",
		TTSChunkDelay:   10 * time.Millisecond,
		TTSChunkCount:   10,
		TTSChunkSize:    3200,
	}

	// Record baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()
	t.Logf("baseline goroutines: %d", baselineGoroutines)

	// Create sessions with ring buffers
	sessions := make([]*session.Session, soakSessions)
	for i := 0; i < soakSessions; i++ {
		id := fmt.Sprintf("soak-session-%d", i)
		sess := session.New(id, cfg.RingBufferSec, logger)
		sessions[i] = sess
	}

	// Start writing audio to ring buffers (simulate inbound RTP)
	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	for _, sess := range sessions {
		wg.Add(1)
		go func(s *session.Session) {
			defer wg.Done()
			silence := make([]byte, 3200) // 100ms at 16kHz s16le
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					s.RingBuffer.Write(silence)
				}
			}
		}(sess)
	}

	// Wait for ring buffers to fill
	time.Sleep(2 * time.Second)

	// Start enunciate loop for each session
	for _, sess := range sessions {
		wg.Add(1)
		go func(s *session.Session) {
			defer wg.Done()
			ticker := time.NewTicker(enunciateInterval)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					// Simulate enunciate: snapshot + mock transcribe + mock TTS
					pcm := s.RingBuffer.Snapshot(5)
					if len(pcm) == 0 {
						continue
					}

					ctx := s.TryStartAction("soak-action", time.Duration(cfg.ActionTimeoutSec)*time.Second)

					_, err := mockClient.Transcribe(ctx, pcm, s.ID, "soak-action", "", "transcribe")
					if err != nil {
						s.FinishAction("soak-action")
						continue
					}

					chunks, errs := mockClient.SynthesizeStream(ctx, "hello world", s.ID, "soak-action", "default", "en", 1.0)
					for range chunks {
					}
					select {
					case <-errs:
					default:
					}

					s.FinishAction("soak-action")
				}
			}
		}(sess)
	}

	// Run for soak duration, sampling goroutines + memory periodically
	deadline := time.Now().Add(soakDuration)
	var memSamples []uint64
	sampleTicker := time.NewTicker(15 * time.Second)
	defer sampleTicker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-sampleTicker.C:
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			goroutines := runtime.NumGoroutine()
			memSamples = append(memSamples, ms.HeapInuse)
			t.Logf("goroutines=%d heapInuse=%dKB heapSys=%dKB",
				goroutines, ms.HeapInuse/1024, ms.HeapSys/1024)
		default:
			time.Sleep(1 * time.Second)
		}
	}

	// Stop everything
	close(stopCh)
	wg.Wait()

	for _, sess := range sessions {
		sess.Stop()
	}

	// Give goroutines time to drain
	time.Sleep(2 * time.Second)
	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	// Assert goroutine count returned to near baseline
	testutil.AssertNoGoroutineLeaks(t, baselineGoroutines, 10)

	// Assert memory is not growing monotonically
	if len(memSamples) >= 4 {
		firstAvg := (memSamples[0] + memSamples[1]) / 2
		lastAvg := (memSamples[len(memSamples)-1] + memSamples[len(memSamples)-2]) / 2
		ratio := float64(lastAvg) / float64(firstAvg)
		t.Logf("memory ratio (last/first avg): %.2f", ratio)
		if ratio > 3.0 {
			t.Errorf("possible memory leak: first avg=%dKB, last avg=%dKB, ratio=%.2f",
				firstAvg/1024, lastAvg/1024, ratio)
		}
	}

	t.Log("soak test completed successfully")
}
