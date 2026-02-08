package ingest

import "context"

// State constants for ingest source lifecycle.
const (
	StateStarting = "starting"
	StateRunning  = "running"
	StateStopped  = "stopped"
	StateError    = "error"
)

// Source is the interface for any audio ingest source (URL, file, etc.).
type Source interface {
	// Start begins ingesting audio. Blocks until ctx is cancelled,
	// the source ends, or an error occurs.
	Start(ctx context.Context) error
	// Stop terminates the ingest. Idempotent.
	Stop()
	// Status returns a snapshot of current ingest state.
	Status() Status
}

// Status describes the current state of an ingest source.
type Status struct {
	State           string  `json:"state"`
	SourceURL       string  `json:"sourceUrl"`
	SecondsBuffered float64 `json:"secondsBuffered"`
	BytesRead       int64   `json:"bytesRead"`
	LastError       string  `json:"lastError,omitempty"`
}
