package applemusic

import (
	"sync"
	"time"
)

// defaultRefreshBuffer is how long before expiry we consider a token stale.
// A 24h buffer on a 30-day token keeps signing rare while avoiding hand-offs
// of nearly-expired tokens to clients.
const defaultRefreshBuffer = 24 * time.Hour

// tokenSigner is the minimal signer surface Cache depends on.
// Small interface kept local to point of use (see golang-patterns).
type tokenSigner interface {
	Sign() (token string, expiresAt time.Time, err error)
}

// Cache wraps a Signer with an in-memory, thread-safe TTL cache.
//
// Concurrent callers share a single cached token until the time remaining
// until expiry drops below refreshBuffer, at which point exactly one caller
// re-signs while any racing callers return the (still-valid) cached token.
type Cache struct {
	signer        tokenSigner
	refreshBuffer time.Duration

	mu        sync.RWMutex
	token     string
	expiresAt time.Time

	// now is injectable for tests. Defaults to time.Now.
	now func() time.Time
}

// NewCache builds a Cache around s. If refreshBuffer <= 0, a 24h default is used.
func NewCache(s *Signer, refreshBuffer time.Duration) *Cache {
	if refreshBuffer <= 0 {
		refreshBuffer = defaultRefreshBuffer
	}
	return &Cache{
		signer:        s,
		refreshBuffer: refreshBuffer,
		now:           time.Now,
	}
}

// Get returns a cached token if its remaining lifetime exceeds refreshBuffer.
// Otherwise it signs a fresh token, caches it, and returns that.
func (c *Cache) Get() (string, time.Time, error) {
	// Fast path: token is present and fresh enough.
	c.mu.RLock()
	token, exp := c.token, c.expiresAt
	c.mu.RUnlock()
	if token != "" && exp.Sub(c.now()) > c.refreshBuffer {
		return token, exp, nil
	}

	// Slow path: take the write lock, re-check (another goroutine may have won),
	// then sign.
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && c.expiresAt.Sub(c.now()) > c.refreshBuffer {
		return c.token, c.expiresAt, nil
	}

	newToken, newExp, err := c.signer.Sign()
	if err != nil {
		return "", time.Time{}, err
	}
	c.token = newToken
	c.expiresAt = newExp
	return newToken, newExp, nil
}
