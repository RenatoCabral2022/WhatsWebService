// Package applemusic signs Apple MusicKit developer tokens (ES256 JWT).
//
// A developer token proves the request is issued by our Apple Developer account.
// It is required by every call to api.music.apple.com. The private key is
// loaded once at startup from an Apple .p8 file; it must never leave the server.
package applemusic

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MaxTokenTTL is Apple's hard cap (6 months) per the MusicKit documentation.
const MaxTokenTTL = 180 * 24 * time.Hour

// Signer creates signed ES256 JWTs usable as Apple MusicKit developer tokens.
type Signer struct {
	teamID     string
	keyID      string
	tokenTTL   time.Duration
	privateKey *ecdsa.PrivateKey
	now        func() time.Time // injectable for tests
}

// Config holds all inputs required to build a Signer.
type Config struct {
	TeamID        string        // Apple Developer Team ID (10 chars)
	KeyID         string        // Key ID of the .p8 (10 chars)
	PrivateKeyPEM []byte        // PEM-encoded PKCS#8 ECDSA private key (.p8 file bytes)
	TokenTTL      time.Duration // How long each signed token is valid
}

// NewSigner parses the PEM key and builds a Signer.
// Returns an error if any field is missing, if the PEM cannot be parsed,
// or if the key is not an ECDSA P-256 key.
func NewSigner(cfg Config) (*Signer, error) {
	if cfg.TeamID == "" {
		return nil, errors.New("applemusic: TeamID is required")
	}
	if cfg.KeyID == "" {
		return nil, errors.New("applemusic: KeyID is required")
	}
	if len(cfg.PrivateKeyPEM) == 0 {
		return nil, errors.New("applemusic: PrivateKeyPEM is required")
	}
	if cfg.TokenTTL <= 0 {
		return nil, errors.New("applemusic: TokenTTL must be positive")
	}
	if cfg.TokenTTL > MaxTokenTTL {
		return nil, fmt.Errorf("applemusic: TokenTTL %s exceeds Apple max %s", cfg.TokenTTL, MaxTokenTTL)
	}

	block, _ := pem.Decode(cfg.PrivateKeyPEM)
	if block == nil {
		return nil, errors.New("applemusic: failed to decode PEM block")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("applemusic: parse PKCS8: %w", err)
	}

	ecKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("applemusic: expected ECDSA key, got %T", parsed)
	}

	return &Signer{
		teamID:     cfg.TeamID,
		keyID:      cfg.KeyID,
		tokenTTL:   cfg.TokenTTL,
		privateKey: ecKey,
		now:        time.Now,
	}, nil
}

// Sign returns a signed ES256 JWT and its absolute expiry.
//
// Header: { alg: ES256, kid: <keyID> }
// Claims: { iss: <teamID>, iat: <now>, exp: <now + TTL> }
func (s *Signer) Sign() (token string, expiresAt time.Time, err error) {
	issuedAt := s.now().UTC()
	expiresAt = issuedAt.Add(s.tokenTTL)

	claims := jwt.MapClaims{
		"iss": s.teamID,
		"iat": issuedAt.Unix(),
		"exp": expiresAt.Unix(),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	t.Header["kid"] = s.keyID

	signed, err := t.SignedString(s.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("applemusic: sign: %w", err)
	}
	return signed, expiresAt, nil
}

// KeyID exposes the configured key id (useful for logging and diagnostics).
func (s *Signer) KeyID() string { return s.keyID }

// TeamID exposes the configured team id (useful for logging and diagnostics).
func (s *Signer) TeamID() string { return s.teamID }
