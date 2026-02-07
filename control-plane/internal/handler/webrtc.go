package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RenatoCabral2022/WHATS-SERVICE/control-plane/internal/model"
)

// PostWebRTCAnswer handles POST /v1/sessions/{sessionId}/webrtc/answer.
func PostWebRTCAnswer(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "sessionId")

	var req model.WebRTCAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// TODO: forward SDP answer to gateway
	w.WriteHeader(http.StatusNoContent)
}
