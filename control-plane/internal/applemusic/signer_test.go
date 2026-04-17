package applemusic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// generateTestKeyPEM returns PKCS#8 PEM bytes of a fresh ephemeral ECDSA P-256 key.
// Intended for tests only; never use a test key for production signing.
func generateTestKeyPEM(t *testing.T) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), priv
}

func TestNewSigner_RejectsMissingFields(t *testing.T) {
	t.Parallel()
	goodPEM, _ := generateTestKeyPEM(t)

	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "missing team id",
			cfg:  Config{KeyID: "K", PrivateKeyPEM: goodPEM, TokenTTL: time.Hour},
		},
		{
			name: "missing key id",
			cfg:  Config{TeamID: "T", PrivateKeyPEM: goodPEM, TokenTTL: time.Hour},
		},
		{
			name: "missing pem",
			cfg:  Config{TeamID: "T", KeyID: "K", TokenTTL: time.Hour},
		},
		{
			name: "zero ttl",
			cfg:  Config{TeamID: "T", KeyID: "K", PrivateKeyPEM: goodPEM},
		},
		{
			name: "ttl over max",
			cfg:  Config{TeamID: "T", KeyID: "K", PrivateKeyPEM: goodPEM, TokenTTL: MaxTokenTTL + time.Second},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, err := NewSigner(tc.cfg)
			if err == nil {
				t.Fatalf("expected error, got signer=%v", s)
			}
		})
	}
}

func TestNewSigner_RejectsMalformedPEM(t *testing.T) {
	t.Parallel()
	_, err := NewSigner(Config{
		TeamID:        "T",
		KeyID:         "K",
		PrivateKeyPEM: []byte("not a pem block at all"),
		TokenTTL:      time.Hour,
	})
	if err == nil {
		t.Fatal("expected error on malformed PEM, got nil")
	}
}

func TestNewSigner_RejectsNonECDSAKey(t *testing.T) {
	t.Parallel()
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	_, err = NewSigner(Config{
		TeamID:        "T",
		KeyID:         "K",
		PrivateKeyPEM: pemBytes,
		TokenTTL:      time.Hour,
	})
	if err == nil {
		t.Fatal("expected error on RSA key, got nil")
	}
}

func TestSign_ProducesValidES256Token(t *testing.T) {
	t.Parallel()
	pemBytes, priv := generateTestKeyPEM(t)

	const teamID = "TEAM123456"
	const keyID = "KEYID67890"
	ttl := 10 * time.Minute

	s, err := NewSigner(Config{
		TeamID:        teamID,
		KeyID:         keyID,
		PrivateKeyPEM: pemBytes,
		TokenTTL:      ttl,
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	before := time.Now().UTC()
	tokenStr, expiresAt, err := s.Sign()
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	after := time.Now().UTC()

	parsed, err := jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, func(tok *jwt.Token) (interface{}, error) {
		if tok.Method.Alg() != "ES256" {
			return nil, jwt.ErrTokenUnverifiable
		}
		return &priv.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("token reported invalid")
	}

	if alg, _ := parsed.Header["alg"].(string); alg != "ES256" {
		t.Errorf("header alg: got %q want ES256", alg)
	}
	if kid, _ := parsed.Header["kid"].(string); kid != keyID {
		t.Errorf("header kid: got %q want %q", kid, keyID)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims wrong type: %T", parsed.Claims)
	}
	if iss, _ := claims["iss"].(string); iss != teamID {
		t.Errorf("iss: got %q want %q", iss, teamID)
	}

	iat, ok := claims["iat"].(float64)
	if !ok {
		t.Fatalf("iat missing or wrong type: %T", claims["iat"])
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("exp missing or wrong type: %T", claims["exp"])
	}

	// 2-second tolerance on either side.
	if int64(iat) < before.Add(-2*time.Second).Unix() || int64(iat) > after.Add(2*time.Second).Unix() {
		t.Errorf("iat %d outside tolerance window [%d, %d]", int64(iat), before.Unix(), after.Unix())
	}
	if got, want := int64(exp), int64(iat)+int64(ttl/time.Second); got != want {
		t.Errorf("exp: got %d want %d (iat + ttl)", got, want)
	}
	if expiresAt.Unix() != int64(exp) {
		t.Errorf("returned expiresAt %d disagrees with claim exp %d", expiresAt.Unix(), int64(exp))
	}
}

func TestSign_ExpiryHonorsTTL(t *testing.T) {
	t.Parallel()
	pemBytes, _ := generateTestKeyPEM(t)

	ttl := 42 * time.Minute
	s, err := NewSigner(Config{
		TeamID:        "T",
		KeyID:         "K",
		PrivateKeyPEM: pemBytes,
		TokenTTL:      ttl,
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	fixed := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	s.now = func() time.Time { return fixed }

	_, expiresAt, err := s.Sign()
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !expiresAt.Equal(fixed.Add(ttl)) {
		t.Errorf("expiresAt: got %s want %s", expiresAt, fixed.Add(ttl))
	}
}
