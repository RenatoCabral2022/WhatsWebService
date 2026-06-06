package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient_RequiresAPIKey(t *testing.T) {
	t.Parallel()
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("expected error when APIKey is empty")
	}
}

func TestTranslate_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path: got %q want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth header: got %q want %q", got, "Bearer test-key")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["model"] != "gpt-4o-mini" {
			t.Errorf("model: got %v want gpt-4o-mini", body["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{"content": "Olá mundo"},
				},
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient(Config{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, err := c.Translate(context.Background(), "hello world", "en", "pt")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if got != "Olá mundo" {
		t.Errorf("translation: got %q want %q", got, "Olá mundo")
	}
}

func TestTranslate_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"oops"}}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{APIKey: "test-key", BaseURL: srv.URL})
	_, err := c.Translate(context.Background(), "x", "en", "pt")
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status: %v", err)
	}
}

func TestTranslate_NoChoices(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer srv.Close()

	c, _ := NewClient(Config{APIKey: "test-key", BaseURL: srv.URL})
	_, err := c.Translate(context.Background(), "x", "en", "pt")
	if err == nil {
		t.Fatal("expected error when no choices returned")
	}
}

func TestSynthesizeStream_Success(t *testing.T) {
	t.Parallel()
	const audio = "fake-mp3-bytes"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["voice"] != "nova" {
			t.Errorf("voice: got %v want nova", body["voice"])
		}
		if body["response_format"] != "mp3" {
			t.Errorf("format: got %v want mp3", body["response_format"])
		}
		if body["speed"] != 1.25 {
			t.Errorf("speed: got %v want 1.25", body["speed"])
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = io.WriteString(w, audio)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{APIKey: "test-key", BaseURL: srv.URL})
	rc, ct, err := c.SynthesizeStream(context.Background(), SynthesizeArgs{
		Text:   "hello",
		Voice:  "nova",
		Format: "mp3",
		Speed:  1.25,
	})
	if err != nil {
		t.Fatalf("SynthesizeStream: %v", err)
	}
	defer rc.Close()
	if ct != "audio/mpeg" {
		t.Errorf("content-type: got %q want audio/mpeg", ct)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != audio {
		t.Errorf("bytes: got %q want %q", got, audio)
	}
}

func TestSynthesizeStream_DefaultsSpeedToOne(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["speed"] != 1.0 {
			t.Errorf("speed default: got %v want 1.0", body["speed"])
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = io.WriteString(w, "x")
	}))
	defer srv.Close()

	c, _ := NewClient(Config{APIKey: "test-key", BaseURL: srv.URL})
	rc, _, err := c.SynthesizeStream(context.Background(), SynthesizeArgs{
		Text: "hi", Voice: "nova", Format: "mp3",
		// Speed omitted on purpose.
	})
	if err != nil {
		t.Fatalf("SynthesizeStream: %v", err)
	}
	_ = rc.Close()
}

func TestSynthesizeStream_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad voice"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{APIKey: "test-key", BaseURL: srv.URL})
	rc, _, err := c.SynthesizeStream(context.Background(), SynthesizeArgs{
		Text: "x", Voice: "bogus", Format: "mp3",
	})
	if err == nil {
		_ = rc.Close()
		t.Fatal("expected error on 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should mention status: %v", err)
	}
}
