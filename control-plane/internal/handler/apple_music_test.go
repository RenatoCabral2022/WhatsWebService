package handler

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/applemusic"
)

func newTestAppleCache(t *testing.T) *applemusic.Cache {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	s, err := applemusic.NewSigner(applemusic.Config{
		TeamID:        "TEAM123456",
		KeyID:         "KEYID67890",
		PrivateKeyPEM: pemBytes,
		TokenTTL:      10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return applemusic.NewCache(s, time.Minute)
}

func TestGetAppleDeveloperToken_NotConfigured(t *testing.T) {
	t.Parallel()
	h := NewHandlers("http://unused.example")

	req := httptest.NewRequest(http.MethodGet, "/v1/music/apple/developer-token", nil)
	rec := httptest.NewRecorder()
	h.GetAppleDeveloperToken(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Errorf("content-type header missing")
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Errorf("expected JSON body, got %q (err=%v)", rec.Body.String(), err)
	}
	if body["error"] == "" {
		t.Errorf("expected error field, got %v", body)
	}
}

func TestGetAppleDeveloperToken_Success(t *testing.T) {
	t.Parallel()
	h := NewHandlers("http://unused.example").WithAppleMusic(newTestAppleCache(t))

	req := httptest.NewRequest(http.MethodGet, "/v1/music/apple/developer-token", nil)
	rec := httptest.NewRecorder()
	h.GetAppleDeveloperToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("Content-Type"), "application/json"; got != want {
		t.Errorf("content-type: got %q want %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("cache-control: got %q want %q", got, want)
	}

	var resp struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Token == "" {
		t.Error("empty token")
	}
	if resp.ExpiresAt <= time.Now().Unix() {
		t.Errorf("expiresAt %d should be in the future", resp.ExpiresAt)
	}
}

// TestGetAppleDeveloperToken_SignerError: the 500 branch on cache.Get() failure
// is exercised by TestCache_SignerErrorPropagates in the applemusic package.
// Reproducing it here would require exporting test hooks across package
// boundaries with no real benefit, so we assert the error-envelope shape via
// the 503 path (same code path for JSON error encoding).
