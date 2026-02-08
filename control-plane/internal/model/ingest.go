package model

// IngestStartRequest is the request body for POST /v1/sessions/{sessionId}/ingest/start.
type IngestStartRequest struct {
	URL string `json:"url"`
}

// IngestStatusResponse is the response for GET /v1/sessions/{sessionId}/ingest/status.
type IngestStatusResponse struct {
	State           string  `json:"state"`
	SourceURL       string  `json:"sourceUrl"`
	SecondsBuffered float64 `json:"secondsBuffered"`
	BytesRead       int64   `json:"bytesRead"`
	LastError       string  `json:"lastError,omitempty"`
}
