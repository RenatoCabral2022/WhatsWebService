package applemusic

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countingSigner implements tokenSigner for tests. Each Sign() returns a
// deterministic token (so callers can assert identity) and increments a counter.
type countingSigner struct {
	mu      sync.Mutex
	calls   int
	ttl     time.Duration
	now     func() time.Time
	signErr error // if non-nil, Sign returns this error
}

func (c *countingSigner) Sign() (string, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.signErr != nil {
		return "", time.Time{}, c.signErr
	}
	c.calls++
	tok := fmt.Sprintf("token-%d", c.calls)
	return tok, c.now().Add(c.ttl), nil
}

func (c *countingSigner) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// newTestCache wires a countingSigner with a shared now function.
func newTestCache(ttl, refresh time.Duration, now func() time.Time) (*Cache, *countingSigner) {
	sig := &countingSigner{ttl: ttl, now: now}
	c := &Cache{
		signer:        sig,
		refreshBuffer: refresh,
		now:           now,
	}
	return c, sig
}

func TestCache_FirstCallSigns(t *testing.T) {
	t.Parallel()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	c, sig := newTestCache(24*time.Hour, time.Hour, func() time.Time { return now })

	tok, exp, err := c.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}
	if !exp.Equal(now.Add(24 * time.Hour)) {
		t.Errorf("exp: got %s want %s", exp, now.Add(24*time.Hour))
	}
	if sig.callCount() != 1 {
		t.Errorf("signer calls: got %d want 1", sig.callCount())
	}
}

func TestCache_SecondCallWithinTTLReusesToken(t *testing.T) {
	t.Parallel()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	c, sig := newTestCache(24*time.Hour, time.Hour, func() time.Time { return now })

	tok1, exp1, err := c.Get()
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	tok2, exp2, err := c.Get()
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if tok1 != tok2 {
		t.Errorf("tokens differ: %q vs %q", tok1, tok2)
	}
	if !exp1.Equal(exp2) {
		t.Errorf("expiries differ: %s vs %s", exp1, exp2)
	}
	if got := sig.callCount(); got != 1 {
		t.Errorf("signer calls: got %d want 1", got)
	}
}

func TestCache_RefreshesWhenNearExpiry(t *testing.T) {
	t.Parallel()
	var nowVal atomic.Int64
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	nowVal.Store(now.Unix())
	clock := func() time.Time { return time.Unix(nowVal.Load(), 0).UTC() }

	sig := &countingSigner{ttl: 24 * time.Hour, now: clock}
	c := &Cache{
		signer:        sig,
		refreshBuffer: time.Hour,
		now:           clock,
	}

	tok1, _, err := c.Get()
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}

	// Advance into the refresh window (23h30m in — less than 1h left).
	nowVal.Store(now.Add(23*time.Hour + 30*time.Minute).Unix())

	tok2, exp2, err := c.Get()
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if tok1 == tok2 {
		t.Errorf("expected new token, got same %q", tok1)
	}
	if exp2.Before(clock().Add(time.Hour)) {
		t.Errorf("refreshed token expires too soon: %s (now=%s)", exp2, clock())
	}
	if got := sig.callCount(); got != 2 {
		t.Errorf("signer calls: got %d want 2", got)
	}
}

func TestCache_SignerErrorPropagates(t *testing.T) {
	t.Parallel()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	sig := &countingSigner{ttl: time.Hour, now: func() time.Time { return now }, signErr: errors.New("boom")}
	c := &Cache{
		signer:        sig,
		refreshBuffer: time.Hour,
		now:           func() time.Time { return now },
	}
	_, _, err := c.Get()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCache_Concurrent(t *testing.T) {
	t.Parallel()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	c, sig := newTestCache(24*time.Hour, time.Hour, func() time.Time { return now })

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)
	tokens := make([]string, goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			tok, _, err := c.Get()
			if err != nil {
				errs <- err
				return
			}
			tokens[i] = tok
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Get error: %v", err)
	}

	if got := sig.callCount(); got != 1 {
		t.Errorf("signer calls under contention: got %d want 1", got)
	}
	first := tokens[0]
	for i, tok := range tokens {
		if tok != first {
			t.Errorf("goroutine %d saw %q, first was %q", i, tok, first)
			break
		}
	}
}

func TestNewCache_DefaultsRefreshBuffer(t *testing.T) {
	t.Parallel()
	s := &Signer{} // unused; we only check the public constructor's defaulting
	c := NewCache(s, 0)
	if c.refreshBuffer != defaultRefreshBuffer {
		t.Errorf("refreshBuffer: got %s want %s", c.refreshBuffer, defaultRefreshBuffer)
	}
	c2 := NewCache(s, 5*time.Minute)
	if c2.refreshBuffer != 5*time.Minute {
		t.Errorf("refreshBuffer: got %s want 5m", c2.refreshBuffer)
	}
}
