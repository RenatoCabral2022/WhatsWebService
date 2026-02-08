package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/RenatoCabral2022/WHATS-SERVICE/control-plane/internal/model"
)

// Handlers holds dependencies for HTTP handlers.
type Handlers struct {
	GatewayBaseURL string
	httpClient     *http.Client
}

// NewHandlers creates handlers that proxy to the gateway internal API.
func NewHandlers(gatewayBaseURL string) *Handlers {
	return &Handlers{
		GatewayBaseURL: gatewayBaseURL,
		httpClient:     &http.Client{Timeout: 15 * 1e9}, // 15 seconds
	}
}

// CreateSession handles POST /v1/sessions.
// Generates a UUID, calls the gateway to create a WebRTC session, and returns the SDP offer.
func (h *Handlers) CreateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := uuid.New().String()

	// Call gateway internal API
	reqBody, _ := json.Marshal(map[string]string{"sessionId": sessionID})
	gwResp, err := h.httpClient.Post(
		h.GatewayBaseURL+"/internal/sessions",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gateway unavailable: %s"}`, err), http.StatusBadGateway)
		return
	}
	defer gwResp.Body.Close()

	if gwResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(gwResp.Body)
		http.Error(w, fmt.Sprintf(`{"error":"gateway error: %s"}`, string(body)), http.StatusBadGateway)
		return
	}

	var gwResult struct {
		SDPOffer   string          `json:"sdpOffer"`
		ICEServers []model.IceServer `json:"iceServers"`
	}
	if err := json.NewDecoder(gwResp.Body).Decode(&gwResult); err != nil {
		http.Error(w, `{"error":"invalid gateway response"}`, http.StatusInternalServerError)
		return
	}

	resp := model.CreateSessionResponse{
		SessionID:  sessionID,
		SdpOffer:   gwResult.SDPOffer,
		IceServers: gwResult.ICEServers,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// DeleteSession handles DELETE /v1/sessions/{sessionId}.
func (h *Handlers) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	req, _ := http.NewRequestWithContext(r.Context(), http.MethodDelete,
		h.GatewayBaseURL+"/internal/sessions/"+sessionID, nil)
	h.httpClient.Do(req) // best-effort

	w.WriteHeader(http.StatusNoContent)
}
