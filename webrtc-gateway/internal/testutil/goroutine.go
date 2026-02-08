package testutil

import (
	"runtime"
	"testing"
	"time"
)

// AssertNoGoroutineLeaks checks that the goroutine count returns to baseline within a deadline.
func AssertNoGoroutineLeaks(t *testing.T, baseline int, margin int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		current := runtime.NumGoroutine()
		if current <= baseline+margin {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Errorf("goroutine leak: baseline=%d, current=%d, margin=%d", baseline, runtime.NumGoroutine(), margin)
}
