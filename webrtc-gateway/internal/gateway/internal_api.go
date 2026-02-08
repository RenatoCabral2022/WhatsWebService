package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/datachannel"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/ingest"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/metrics"
)

type createSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type createSessionResponse struct {
	SDPOffer   string      `json:"sdpOffer"`
	ICEServers []iceServer `json:"iceServers"`
}

type iceServer struct {
	URLs []string `json:"urls"`
}

type answerRequest struct {
	SDPAnswer string `json:"sdpAnswer"`
}

type ingestStartRequest struct {
	URL string `json:"url"`
}

type ingestStatusResponse struct {
	State           string  `json:"state"`
	SourceURL       string  `json:"sourceUrl"`
	SecondsBuffered float64 `json:"secondsBuffered"`
	BytesRead       int64   `json:"bytesRead"`
	LastError       string  `json:"lastError,omitempty"`
}

// InternalHandler returns an http.Handler for the gateway's internal API.
func (gw *Gateway) InternalHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/sessions", gw.handleCreateSession)
	mux.HandleFunc("/internal/sessions/", gw.handleSessionRoutes)
	return mux
}

func (gw *Gateway) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req createSessionRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &req); err != nil || req.SessionID == "" {
		http.Error(w, "invalid request: sessionId required", http.StatusBadRequest)
		return
	}

	// Enforce session cap
	if count := gw.SessionCount(); count >= gw.cfg.MaxSessions {
		gw.logger.Warn("session cap reached", zap.Int("current", count), zap.Int("max", gw.cfg.MaxSessions))
		metrics.SessionsRejectedTotal.Inc()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "max sessions reached"})
		return
	}

	sdpOffer, err := gw.CreateSession(req.SessionID)
	if err != nil {
		gw.logger.Error("create session failed", zap.Error(err))
		http.Error(w, "create session failed", http.StatusInternalServerError)
		return
	}

	// Build ICE servers response
	iceServers := make([]iceServer, 0, len(gw.cfg.STUNServers))
	if len(gw.cfg.STUNServers) > 0 {
		iceServers = append(iceServers, iceServer{URLs: gw.cfg.STUNServers})
	}

	resp := createSessionResponse{
		SDPOffer:   sdpOffer,
		ICEServers: iceServers,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (gw *Gateway) handleSessionRoutes(w http.ResponseWriter, r *http.Request) {
	// Parse: /internal/sessions/{id} or /internal/sessions/{id}/webrtc/answer
	path := strings.TrimPrefix(r.URL.Path, "/internal/sessions/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "session id required", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]

	if len(parts) == 1 {
		// DELETE /internal/sessions/{id}
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		gw.DeleteSession(sessionID)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	suffix := parts[1]

	// POST /internal/sessions/{id}/webrtc/answer
	if suffix == "webrtc/answer" && r.Method == http.MethodPost {
		gw.handleSetAnswer(w, r, sessionID)
		return
	}

	// Ingest routes
	if suffix == "ingest/start" && r.Method == http.MethodPost {
		gw.handleIngestStart(w, r, sessionID)
		return
	}
	if suffix == "ingest/stop" && r.Method == http.MethodPost {
		gw.handleIngestStop(w, r, sessionID)
		return
	}
	if suffix == "ingest/status" && r.Method == http.MethodGet {
		gw.handleIngestStatus(w, r, sessionID)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func (gw *Gateway) handleSetAnswer(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req answerRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &req); err != nil || req.SDPAnswer == "" {
		http.Error(w, "invalid request: sdpAnswer required", http.StatusBadRequest)
		return
	}

	if err := gw.SetAnswer(sessionID, req.SDPAnswer); err != nil {
		if strings.Contains(err.Error(), "session not found") {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		gw.logger.Error("set answer failed", zap.Error(err))
		http.Error(w, "set answer failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (gw *Gateway) handleIngestStart(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req ingestStartRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &req); err != nil || req.URL == "" {
		http.Error(w, `{"error":"url is required"}`, http.StatusBadRequest)
		return
	}
	req.URL = strings.TrimSpace(req.URL)

	// SSRF validation
	if err := ingest.ValidateURL(req.URL); err != nil {
		gw.logger.Warn("ingest URL rejected", zap.String("url", req.URL), zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	gw.mu.RLock()
	sess, ok := gw.sessions[sessionID]
	gw.mu.RUnlock()
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Stop any existing ingest
	sess.StopIngest()

	// Create and attach new ingest source
	src := ingest.NewFFmpegURLSource(req.URL, sess.RingBuffer,
		gw.cfg.MaxIngestDurationSec, gw.logger)
	sess.SetIngestSource(src)

	// Send ingest.started event over data channel
	startedPayload, _ := json.Marshal(datachannel.EventIngestStarted{URL: req.URL})
	sess.SendDataChannelMessage(datachannel.Envelope{
		Type:      "ingest.started",
		SessionID: sessionID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   json.RawMessage(startedPayload),
	})

	// Start ingest in background goroutine
	metrics.IngestsStartedTotal.Inc()
	metrics.ActiveIngests.Inc()
	go func() {
		defer metrics.ActiveIngests.Dec()
		if err := src.Start(context.Background()); err != nil {
			gw.logger.Warn("ingest ended with error",
				zap.String("session", sessionID), zap.Error(err))
			gw.sendError(sess, sessionID, "", "INGEST_FAILED", err.Error())
			metrics.IngestsFailedTotal.Inc()
		}
		// Send ingest.stopped event
		stoppedPayload, _ := json.Marshal(datachannel.EventIngestStopped{Reason: "source_ended"})
		sess.SendDataChannelMessage(datachannel.Envelope{
			Type:      "ingest.stopped",
			SessionID: sessionID,
			Timestamp: time.Now().UnixMilli(),
			Payload:   json.RawMessage(stoppedPayload),
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (gw *Gateway) handleIngestStop(w http.ResponseWriter, r *http.Request, sessionID string) {
	gw.mu.RLock()
	sess, ok := gw.sessions[sessionID]
	gw.mu.RUnlock()
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	sess.StopIngest()
	w.WriteHeader(http.StatusNoContent)
}

func (gw *Gateway) handleIngestStatus(w http.ResponseWriter, r *http.Request, sessionID string) {
	gw.mu.RLock()
	sess, ok := gw.sessions[sessionID]
	gw.mu.RUnlock()
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	status := sess.IngestStatus()
	if status == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ingestStatusResponse{State: "none"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ingestStatusResponse{
		State:           status.State,
		SourceURL:       status.SourceURL,
		SecondsBuffered: status.SecondsBuffered,
		BytesRead:       status.BytesRead,
		LastError:       status.LastError,
	})
}
