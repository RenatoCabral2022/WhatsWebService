package handler

import (
	"encoding/json"
	"net/http"

	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/applemusic"
)

// WithAppleMusic attaches an Apple Music token cache to the handler set.
// Returns the same *Handlers so it can be chained fluently in main.
// When the cache is nil (not configured), the developer-token endpoint returns 503.
func (h *Handlers) WithAppleMusic(cache *applemusic.Cache) *Handlers {
	h.AppleMusic = cache
	return h
}

// GetAppleDeveloperToken handles GET /v1/music/apple/developer-token.
//
// Returns an ES256 JWT signed with the server's Apple MusicKit key, suitable
// for use as the Authorization bearer on api.music.apple.com.
//
// The token is cached server-side and reused across callers until it nears
// expiry. The Cache-Control header permits private (per-user) caching for
// one hour, which bounds client-side reuse without sharing across users.
func (h *Handlers) GetAppleDeveloperToken(w http.ResponseWriter, r *http.Request) {
	if h.AppleMusic == nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"apple music not configured"}`, http.StatusServiceUnavailable)
		return
	}

	token, expiresAt, err := h.AppleMusic.Get()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"failed to sign apple developer token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_ = json.NewEncoder(w).Encode(struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expiresAt"`
	}{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
	})
}
