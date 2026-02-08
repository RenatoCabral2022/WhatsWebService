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

// PostWebRTCAnswer handles POST /v1/sessions/{sessionId}/webrtc/answer.
// Proxies the SDP answer to the gateway internal API.
func (h *Handlers) PostWebRTCAnswer(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	var req model.WebRTCAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.SdpAnswer == "" {
		http.Error(w, `{"error":"sdpAnswer is required"}`, http.StatusBadRequest)
		return
	}

	reqBody, _ := json.Marshal(map[string]string{"sdpAnswer": req.SdpAnswer})
	gwResp, err := h.httpClient.Post(
		fmt.Sprintf("%s/internal/sessions/%s/webrtc/answer", h.GatewayBaseURL, sessionID),
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gateway unavailable: %s"}`, err), http.StatusBadGateway)
		return
	}
	defer gwResp.Body.Close()

	if gwResp.StatusCode == http.StatusNotFound {
		http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
		return
	}
	if gwResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(gwResp.Body)
		http.Error(w, fmt.Sprintf(`{"error":"gateway error: %s"}`, string(body)), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
