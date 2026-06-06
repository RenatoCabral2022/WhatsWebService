// Package openai is a tiny, hosting-portable client for the OpenAI HTTP API
// covering only the two surfaces the control-plane needs: translation via
// chat completions and streaming text-to-speech.
//
// No SDK dependency, no provider-specific bindings — plain net/http so a
// future move off GCP onto cheaper hosting requires no changes here.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const defaultBaseURL = "https://api.openai.com"

// HTTPDoer is the minimal HTTP surface Client depends on. Tests inject a stub
// so the handler tests never hit the real OpenAI API.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client talks to OpenAI's REST API. Construct with NewClient.
type Client struct {
	apiKey         string
	translateModel string
	ttsModel       string
	baseURL        string
	http           HTTPDoer
}

// Config is the constructor input for NewClient. APIKey is required;
// everything else takes a sensible default.
type Config struct {
	APIKey         string
	TranslateModel string
	TTSModel       string
	BaseURL        string   // override for tests; defaults to https://api.openai.com
	HTTP           HTTPDoer // override for tests; defaults to http.DefaultClient
}

// NewClient validates the config and returns a ready Client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("openai: APIKey is required")
	}
	if cfg.TranslateModel == "" {
		cfg.TranslateModel = "gpt-4o-mini"
	}
	if cfg.TTSModel == "" {
		cfg.TTSModel = "gpt-4o-mini-tts"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.HTTP == nil {
		cfg.HTTP = http.DefaultClient
	}
	return &Client{
		apiKey:         cfg.APIKey,
		translateModel: cfg.TranslateModel,
		ttsModel:       cfg.TTSModel,
		baseURL:        cfg.BaseURL,
		http:           cfg.HTTP,
	}, nil
}

// Translate returns text rendered in tgtLang. It is a single, buffered call —
// streaming chat completions wouldn't help us here because TTS can't start
// until the full translation is known.
func (c *Client) Translate(ctx context.Context, text, srcLang, tgtLang string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		Temperature float64   `json:"temperature"`
	}{
		Model: c.translateModel,
		Messages: []message{
			{
				Role: "system",
				Content: fmt.Sprintf(
					"You are a precise translator. Translate the user's text from %s to %s. Preserve line breaks. Output only the translation — no preamble, no quotes, no commentary.",
					srcLang, tgtLang,
				),
			},
			{Role: "user", Content: text},
		},
		Temperature: 0,
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("openai: marshal translate request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("openai: build translate request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: translate http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("openai: translate status %d: %s", resp.StatusCode, string(errBody))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("openai: decode translate response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("openai: translate returned no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

// SynthesizeArgs is the input for SynthesizeStream.
type SynthesizeArgs struct {
	Text   string
	Voice  string
	Format string  // "mp3" or "opus"
	Speed  float64 // 0.5..2.0
}

// SynthesizeStream POSTs to /v1/audio/speech and returns the response body so
// the caller can stream it straight to the client. The caller is responsible
// for closing the returned ReadCloser.
//
// On non-2xx OpenAI replies, the response body is read into the returned
// error and the body is closed before returning.
func (c *Client) SynthesizeStream(ctx context.Context, args SynthesizeArgs) (io.ReadCloser, string, error) {
	if args.Speed == 0 {
		args.Speed = 1.0
	}
	body := struct {
		Model          string  `json:"model"`
		Input          string  `json:"input"`
		Voice          string  `json:"voice"`
		ResponseFormat string  `json:"response_format"`
		Speed          float64 `json:"speed"`
	}{
		Model:          c.ttsModel,
		Input:          args.Text,
		Voice:          args.Voice,
		ResponseFormat: args.Format,
		Speed:          args.Speed,
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("openai: marshal tts request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/audio/speech", bytes.NewReader(buf))
	if err != nil {
		return nil, "", fmt.Errorf("openai: build tts request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", contentTypeFor(args.Format))

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("openai: tts http: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("openai: tts status %d: %s", resp.StatusCode, string(errBody))
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = contentTypeFor(args.Format)
	}
	return resp.Body, ct, nil
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
