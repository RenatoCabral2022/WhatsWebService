package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/openai"
	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/ttscache"
)

// fakeOpenAI is a hand-rolled OpenAITTS stub that lets tests control return
// values and observe call counts without going through net/http.
type fakeOpenAI struct {
	translateResp string
	translateErr  error
	translateHits int32

	synthResp    string
	synthCT      string
	synthErr     error
	synthHits    int32
	gotSynthArgs openai.SynthesizeArgs
}

func (f *fakeOpenAI) Translate(ctx context.Context, text, src, tgt string) (string, error) {
	atomic.AddInt32(&f.translateHits, 1)
	if f.translateErr != nil {
		return "", f.translateErr
	}
	return f.translateResp, nil
}

func (f *fakeOpenAI) SynthesizeStream(ctx context.Context, args openai.SynthesizeArgs) (io.ReadCloser, string, error) {
	atomic.AddInt32(&f.synthHits, 1)
	f.gotSynthArgs = args
	if f.synthErr != nil {
		return nil, "", f.synthErr
	}
	ct := f.synthCT
	if ct == "" {
		ct = "audio/mpeg"
	}
	return io.NopCloser(strings.NewReader(f.synthResp)), ct, nil
}

func newTTSHandler(t *testing.T, fo *fakeOpenAI) *Handlers {
	t.Helper()
	cache, err := ttscache.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	return NewHandlers("http://unused.example").WithOpenAI(fo, cache, TTSConfig{
		DefaultVoice:  "nova",
		DefaultFormat: "mp3",
	})
}

func TestPostTTSEnunciate_NotConfigured(t *testing.T) {
	t.Parallel()
	h := NewHandlers("http://unused.example") // no WithOpenAI

	req := httptest.NewRequest(http.MethodPost, "/v1/tts/enunciate", strings.NewReader(`{"text":"hi","sourceLanguage":"en"}`))
	rec := httptest.NewRecorder()
	h.PostTTSEnunciate(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want 503", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] == "" {
		t.Errorf("expected error field")
	}
}

func TestPostTTSEnunciate_RejectsInvalidJSON(t *testing.T) {
	t.Parallel()
	h := newTTSHandler(t, &fakeOpenAI{synthResp: "x"})
	req := httptest.NewRequest(http.MethodPost, "/v1/tts/enunciate", strings.NewReader(`{not json`))
	rec := httptest.NewRecorder()
	h.PostTTSEnunciate(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", rec.Code)
	}
}

func TestPostTTSEnunciate_RejectsMissingFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{"missing text", `{"sourceLanguage":"en"}`},
		{"missing source lang", `{"text":"hello"}`},
		{"whitespace text", `{"text":"   ","sourceLanguage":"en"}`},
		{"speed too high", `{"text":"x","sourceLanguage":"en","speed":3.0}`},
		{"speed too low", `{"text":"x","sourceLanguage":"en","speed":0.1}`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newTTSHandler(t, &fakeOpenAI{synthResp: "x"})
			req := httptest.NewRequest(http.MethodPost, "/v1/tts/enunciate", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			h.PostTTSEnunciate(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("body=%q: got %d want 400", tc.body, rec.Code)
			}
		})
	}
}

func TestPostTTSEnunciate_NoTranslation_SynthAndCache(t *testing.T) {
	t.Parallel()
	fo := &fakeOpenAI{synthResp: "audio-bytes-here", synthCT: "audio/mpeg"}
	h := newTTSHandler(t, fo)

	body := `{"text":"hello","sourceLanguage":"en"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/tts/enunciate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.PostTTSEnunciate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("Content-Type"), "audio/mpeg"; got != want {
		t.Errorf("content-type: got %q want %q", got, want)
	}
	if rec.Body.String() != "audio-bytes-here" {
		t.Errorf("body: got %q want %q", rec.Body.String(), "audio-bytes-here")
	}
	if atomic.LoadInt32(&fo.translateHits) != 0 {
		t.Errorf("translate should not be called when targetLanguage is empty")
	}
	if atomic.LoadInt32(&fo.synthHits) != 1 {
		t.Errorf("synth hits: got %d want 1", fo.synthHits)
	}
	if fo.gotSynthArgs.Voice != "nova" || fo.gotSynthArgs.Format != "mp3" {
		t.Errorf("synth args defaults wrong: %+v", fo.gotSynthArgs)
	}

	// Second call must hit the cache (no extra synth).
	rec2 := httptest.NewRecorder()
	h.PostTTSEnunciate(rec2, httptest.NewRequest(http.MethodPost, "/v1/tts/enunciate", strings.NewReader(body)))
	if rec2.Code != http.StatusOK {
		t.Fatalf("cache hit status: got %d", rec2.Code)
	}
	if rec2.Body.String() != "audio-bytes-here" {
		t.Errorf("cache hit body: got %q", rec2.Body.String())
	}
	if atomic.LoadInt32(&fo.synthHits) != 1 {
		t.Errorf("synth called again on second request; cache miss?")
	}
}

func TestPostTTSEnunciate_TranslationPath(t *testing.T) {
	t.Parallel()
	fo := &fakeOpenAI{
		translateResp: "Olá",
		synthResp:     "pt-audio",
		synthCT:       "audio/mpeg",
	}
	h := newTTSHandler(t, fo)

	body := `{"text":"Hello","sourceLanguage":"en","targetLanguage":"pt"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/tts/enunciate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.PostTTSEnunciate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if atomic.LoadInt32(&fo.translateHits) != 1 {
		t.Errorf("translate hits: got %d want 1", fo.translateHits)
	}
	if fo.gotSynthArgs.Text != "Olá" {
		t.Errorf("synth should receive translated text, got %q", fo.gotSynthArgs.Text)
	}
}

func TestPostTTSEnunciate_TranslationError(t *testing.T) {
	t.Parallel()
	fo := &fakeOpenAI{translateErr: errors.New("upstream down"), synthResp: "x"}
	h := newTTSHandler(t, fo)

	req := httptest.NewRequest(http.MethodPost, "/v1/tts/enunciate",
		strings.NewReader(`{"text":"Hello","sourceLanguage":"en","targetLanguage":"pt"}`))
	rec := httptest.NewRecorder()
	h.PostTTSEnunciate(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status: got %d want 502", rec.Code)
	}
	if atomic.LoadInt32(&fo.synthHits) != 0 {
		t.Errorf("synth must not run when translation fails")
	}
}

func TestPostTTSEnunciate_SynthError(t *testing.T) {
	t.Parallel()
	fo := &fakeOpenAI{synthErr: errors.New("upstream tts down")}
	h := newTTSHandler(t, fo)
	req := httptest.NewRequest(http.MethodPost, "/v1/tts/enunciate",
		strings.NewReader(`{"text":"hello","sourceLanguage":"en"}`))
	rec := httptest.NewRecorder()
	h.PostTTSEnunciate(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status: got %d want 502", rec.Code)
	}
}

func TestPostTTSEnunciate_SameSourceAndTarget_SkipsTranslate(t *testing.T) {
	t.Parallel()
	fo := &fakeOpenAI{synthResp: "x"}
	h := newTTSHandler(t, fo)
	req := httptest.NewRequest(http.MethodPost, "/v1/tts/enunciate",
		strings.NewReader(`{"text":"hi","sourceLanguage":"en","targetLanguage":"EN"}`))
	rec := httptest.NewRecorder()
	h.PostTTSEnunciate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	if atomic.LoadInt32(&fo.translateHits) != 0 {
		t.Errorf("translate must be skipped when src == tgt (case-insensitive)")
	}
}
