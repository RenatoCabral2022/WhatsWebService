package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/openai"
	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/ttscache"
)

// Latency budgets. Cold path realism: gpt-4o-mini translation can take 3–6s
// for a 30s lyric window, and gpt-4o-mini-tts adds another 1–3s. We size the
// deadlines so the server fails after the client gives up (≈12s), not before
// — otherwise the 502 forces a fallback to expo-speech right as OpenAI was
// about to reply. Warm-path (cache hit) is unaffected.
const (
	translateDeadline = 8 * time.Second
	ttsDeadline       = 10 * time.Second
	maxTextLen        = 4000 // bytes; safety bound for OpenAI input limit
)

// OpenAITTS is the subset of the OpenAI client the handler depends on,
// kept small so tests can stub it without pulling in the real client.
type OpenAITTS interface {
	Translate(ctx context.Context, text, srcLang, tgtLang string) (string, error)
	SynthesizeStream(ctx context.Context, args openai.SynthesizeArgs) (io.ReadCloser, string, error)
}

// TTSConfig is the boot-time configuration WithOpenAI captures. Defaults
// fill in for omitted request fields.
type TTSConfig struct {
	DefaultVoice  string
	DefaultFormat string // "mp3" or "opus"
}

// WithOpenAI attaches the OpenAI client + cache + defaults to the handler
// set. When the client is nil, /v1/tts/enunciate serves 503.
//
// Mirrors the WithAppleMusic builder pattern already used for Apple Music.
func (h *Handlers) WithOpenAI(client OpenAITTS, cache ttscache.Cache, cfg TTSConfig) *Handlers {
	if cfg.DefaultVoice == "" {
		cfg.DefaultVoice = "nova"
	}
	if cfg.DefaultFormat == "" {
		cfg.DefaultFormat = "mp3"
	}
	h.OpenAI = client
	h.TTSCache = cache
	h.TTS = cfg
	return h
}

// ttsRequest is the JSON body of POST /v1/tts/enunciate.
type ttsRequest struct {
	Text           string   `json:"text"`
	SourceLanguage string   `json:"sourceLanguage"`
	TargetLanguage string   `json:"targetLanguage,omitempty"`
	Voice          string   `json:"voice,omitempty"`
	Speed          *float64 `json:"speed,omitempty"`
	TrackID        string   `json:"trackId,omitempty"`
}

func (req *ttsRequest) validate() error {
	if strings.TrimSpace(req.Text) == "" {
		return errors.New("text is required")
	}
	if len(req.Text) > maxTextLen {
		return fmt.Errorf("text exceeds %d-byte limit", maxTextLen)
	}
	if strings.TrimSpace(req.SourceLanguage) == "" {
		return errors.New("sourceLanguage is required")
	}
	if req.Speed != nil && (*req.Speed < 0.5 || *req.Speed > 2.0) {
		return errors.New("speed must be between 0.5 and 2.0")
	}
	return nil
}

// PostTTSEnunciate handles POST /v1/tts/enunciate.
//
// Pipeline:
//  1. Validate JSON.
//  2. If targetLanguage differs from sourceLanguage, translate via OpenAI.
//  3. Compute a content-hash cache key over the final text + voice + format
//     + speed + targetLanguage.
//  4. Cache hit: stream cached bytes back.
//  5. Cache miss: stream OpenAI TTS to the client and the cache in parallel;
//     only commit the cache entry if the upstream stream finishes cleanly.
//
// On client disconnect mid-stream, the deferred cache Close() removes the
// half-written temp file so we never serve a corrupt cache hit later.
func (h *Handlers) PostTTSEnunciate(w http.ResponseWriter, r *http.Request) {
	if h.OpenAI == nil || h.TTSCache == nil {
		writeTTSError(w, http.StatusServiceUnavailable, "openai tts not configured")
		return
	}

	var req ttsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeTTSError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %s", err))
		return
	}
	if err := req.validate(); err != nil {
		writeTTSError(w, http.StatusBadRequest, err.Error())
		return
	}

	voice := req.Voice
	if voice == "" {
		voice = h.TTS.DefaultVoice
	}
	format := h.TTS.DefaultFormat
	speed := 1.0
	if req.Speed != nil {
		speed = *req.Speed
	}

	// Translation (if requested + source != target).
	text := req.Text
	effectiveLang := req.SourceLanguage
	if req.TargetLanguage != "" && !strings.EqualFold(req.TargetLanguage, req.SourceLanguage) {
		tCtx, cancel := context.WithTimeout(r.Context(), translateDeadline)
		translated, err := h.OpenAI.Translate(tCtx, req.Text, req.SourceLanguage, req.TargetLanguage)
		cancel()
		if err != nil {
			log.Printf("tts: translate failed: %v", err)
			writeTTSError(w, http.StatusBadGateway, "translation failed")
			return
		}
		text = translated
		effectiveLang = req.TargetLanguage
	}

	key := h.TTSCache.Key(text, voice, format, speed, effectiveLang)

	// Cache hit: stream the file straight to the client.
	if rc, size, ok := h.TTSCache.Get(key, format); ok {
		defer rc.Close()
		w.Header().Set("Content-Type", contentTypeFor(format))
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		}
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, rc); err != nil {
			// Client likely disconnected — nothing to recover.
			log.Printf("tts: cache stream copy: %v", err)
		}
		return
	}

	// Cache miss: synthesize.
	sCtx, cancel := context.WithTimeout(r.Context(), ttsDeadline)
	defer cancel()

	body, upstreamCT, err := h.OpenAI.SynthesizeStream(sCtx, openai.SynthesizeArgs{
		Text:   text,
		Voice:  voice,
		Format: format,
		Speed:  speed,
	})
	if err != nil {
		log.Printf("tts: synth failed: %v", err)
		writeTTSError(w, http.StatusBadGateway, "synthesis failed")
		return
	}
	defer body.Close()

	cw, err := h.TTSCache.Open(key, format)
	if err != nil {
		// Caching is best-effort; degrade to client-only stream rather than fail.
		log.Printf("tts: cache open failed (degraded to no-cache): %v", err)
		w.Header().Set("Content-Type", upstreamCT)
		w.WriteHeader(http.StatusOK)
		streamWithFlush(w, body)
		return
	}
	defer cw.Close() // safe after Commit — no-ops then.

	w.Header().Set("Content-Type", upstreamCT)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)

	if err := streamTee(w, cw, body); err != nil {
		log.Printf("tts: tee stream: %v", err)
		// cw.Close (deferred) removes the temp file; no Commit.
		return
	}
	if err := cw.Commit(); err != nil {
		log.Printf("tts: cache commit failed: %v", err)
	}
}

// streamTee copies src into both dst (the HTTP response) and cache. It flushes
// after each chunk so bytes leave the server as soon as OpenAI produces them.
// Errors writing to the cache are logged but do not abort the client stream.
func streamTee(dst http.ResponseWriter, cache io.Writer, src io.Reader) error {
	flusher, _ := dst.(http.Flusher)
	buf := make([]byte, 8*1024)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
			if flusher != nil {
				flusher.Flush()
			}
			if _, cerr := cache.Write(buf[:n]); cerr != nil {
				log.Printf("tts: cache write failed mid-stream: %v", cerr)
				cache = io.Discard
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// streamWithFlush is the no-cache fallback (used when cache.Open fails).
func streamWithFlush(dst http.ResponseWriter, src io.Reader) {
	flusher, _ := dst.(http.Flusher)
	buf := make([]byte, 8*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

func contentTypeFor(format string) string {
	switch format {
	case "opus":
		return "audio/opus"
	case "mp3":
		return "audio/mpeg"
	default:
		return "audio/mpeg"
	}
}

func writeTTSError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
