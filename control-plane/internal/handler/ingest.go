package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/model"
)

// PostIngestStart handles POST /v1/sessions/{sessionId}/ingest/start.
// Proxies the request to the gateway internal API.
func (h *Handlers) PostIngestStart(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	var req model.IngestStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, `{"error":"url is required"}`, http.StatusBadRequest)
		return
	}

	reqBody, _ := json.Marshal(map[string]string{"url": req.URL})
	gwResp, err := h.httpClient.Post(
		fmt.Sprintf("%s/internal/sessions/%s/ingest/start", h.GatewayBaseURL, sessionID),
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gateway unavailable: %s"}`, err), http.StatusBadGateway)
		return
	}
	defer gwResp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(gwResp.StatusCode)
	io.Copy(w, gwResp.Body)
}

// PostIngestStop handles POST /v1/sessions/{sessionId}/ingest/stop.
func (h *Handlers) PostIngestStop(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost,
		fmt.Sprintf("%s/internal/sessions/%s/ingest/stop", h.GatewayBaseURL, sessionID), nil)
	gwResp, err := h.httpClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gateway unavailable: %s"}`, err), http.StatusBadGateway)
		return
	}
	defer gwResp.Body.Close()

	w.WriteHeader(gwResp.StatusCode)
}

// GetIngestStatus handles GET /v1/sessions/{sessionId}/ingest/status.
func (h *Handlers) GetIngestStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet,
		fmt.Sprintf("%s/internal/sessions/%s/ingest/status", h.GatewayBaseURL, sessionID), nil)
	gwResp, err := h.httpClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gateway unavailable: %s"}`, err), http.StatusBadGateway)
		return
	}
	defer gwResp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(gwResp.StatusCode)
	io.Copy(w, gwResp.Body)
}
