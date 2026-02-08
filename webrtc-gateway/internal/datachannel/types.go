package datachannel

import "encoding/json"

// Envelope is the top-level wrapper for all data channel messages.
type Envelope struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionId"`
	ActionID  string          `json:"actionId,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// CommandEnunciate is the payload for command.enunciate messages.
type CommandEnunciate struct {
	LookbackSeconds int        `json:"lookbackSeconds"`
	TargetLanguage  string     `json:"targetLanguage,omitempty"`
	TTSOptions      TTSOptions `json:"ttsOptions,omitempty"`
}

// TTSOptions controls text-to-speech synthesis parameters.
type TTSOptions struct {
	Voice string  `json:"voice,omitempty"`
	Speed float64 `json:"speed,omitempty"`
}

// EventAsrFinal is the payload for asr.final events.
type EventAsrFinal struct {
	Text           string    `json:"text"`
	Language       string    `json:"language"`
	TranslatedText string   `json:"translatedText,omitempty"`
	TargetLanguage string   `json:"targetLanguage,omitempty"`
	Segments       []Segment `json:"segments,omitempty"`
	InferenceMs    int       `json:"inferenceMs,omitempty"`
	TranslateMs    int       `json:"translateMs,omitempty"`
}

// Segment is a time-aligned piece of transcription.
type Segment struct {
	Text       string  `json:"text"`
	StartTime  float64 `json:"startTime"`
	EndTime    float64 `json:"endTime"`
	Confidence float64 `json:"confidence,omitempty"`
}

// EventMetricsLatency is the payload for metrics.latency events.
type EventMetricsLatency struct {
	SnapshotMs     float64 `json:"snapshotMs"`
	AsrMs          float64 `json:"asrMs"`
	TranslateMs    float64 `json:"translateMs,omitempty"`
	TtsFirstChunkMs float64 `json:"ttsFirstChunkMs"`
	TotalMs        float64 `json:"totalMs"`
}

// EventTtsDone is the payload for tts.done events.
type EventTtsDone struct {
	DurationMs int `json:"durationMs"`
}

// EventError is the payload for error events.
type EventError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}
