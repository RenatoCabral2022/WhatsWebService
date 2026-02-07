package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/RenatoCabral2022/WHATS-SERVICE/control-plane/internal/model"
)

// CreateSession handles POST /v1/sessions.
func CreateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := uuid.New().String()

	resp := model.CreateSessionResponse{
		SessionID: sessionID,
		SdpOffer:  "", // TODO: generate via gateway coordination
		IceServers: []model.IceServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// DeleteSession handles DELETE /v1/sessions/{sessionId}.
func DeleteSession(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "sessionId") // TODO: tear down session
	w.WriteHeader(http.StatusNoContent)
}
