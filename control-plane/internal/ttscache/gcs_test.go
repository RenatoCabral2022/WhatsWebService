package ttscache

import (
	"context"
	"io"
	"os"
	"testing"
	"time"
)

// TestGCS_RoundTrip is an opt-in integration test against a real bucket.
//
// Set GCS_TEST_BUCKET to a bucket your ADC can write to to enable.
// Run with: GCS_TEST_BUCKET=my-bucket go test ./internal/ttscache/...
//
// Skipped in normal CI so we don't flake on credentials/network.
func TestGCS_RoundTrip(t *testing.T) {
	bucket := os.Getenv("GCS_TEST_BUCKET")
	if bucket == "" {
		t.Skip("set GCS_TEST_BUCKET to run this test against a real bucket")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := NewGCS(ctx, bucket)
	if err != nil {
		t.Fatalf("NewGCS: %v", err)
	}
	defer c.Close()

	// Unique key per run so reruns don't depend on prior state.
	key := c.Key("hello gcs "+t.Name()+time.Now().UTC().Format(time.RFC3339Nano),
		"nova", "mp3", 1.0, "en")

	// Initial miss.
	if rc, _, ok := c.Get(key, "mp3"); ok {
		_ = rc.Close()
		t.Fatalf("expected miss for fresh key")
	}

	// Write + commit.
	w, err := c.Open(key, "mp3")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	want := []byte("fake-audio-bytes-from-gcs-test")
	if _, err := w.Write(want); err != nil {
		_ = w.Close()
		t.Fatalf("Write: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	_ = w.Close() // no-op after Commit

	// Hit.
	rc, size, ok := c.Get(key, "mp3")
	if !ok {
		t.Fatalf("expected hit after Commit")
	}
	defer rc.Close()
	if size != int64(len(want)) {
		t.Errorf("size: got %d want %d", size, len(want))
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("bytes: got %q want %q", got, want)
	}

	// Abort path: open another key, close without commit, verify miss.
	abortKey := c.Key("aborted "+t.Name()+time.Now().UTC().Format(time.RFC3339Nano),
		"nova", "mp3", 1.0, "en")
	aw, err := c.Open(abortKey, "mp3")
	if err != nil {
		t.Fatalf("Open (abort): %v", err)
	}
	if _, err := aw.Write([]byte("half-data")); err != nil {
		t.Logf("Write before abort: %v (acceptable — context will be canceled on Close)", err)
	}
	if err := aw.Close(); err != nil {
		t.Errorf("Close (abort): %v", err)
	}
	if rc, _, ok := c.Get(abortKey, "mp3"); ok {
		_ = rc.Close()
		t.Errorf("abort path: object materialized in GCS — should not have committed")
	}
}

func TestGCS_ObjectName(t *testing.T) {
	t.Parallel()
	g := &GCS{} // exercising the pure helper only — no GCS calls
	got := g.objectName("abc123", "mp3")
	want := "tts/abc123.mp3"
	if got != want {
		t.Errorf("objectName: got %q want %q", got, want)
	}
}

func TestGCS_ContentType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		format string
		want   string
	}{
		{"mp3", "audio/mpeg"},
		{"opus", "audio/opus"},
		{"", "audio/mpeg"},
		{"unknown", "audio/mpeg"},
	}
	for _, tc := range cases {
		if got := gcsContentType(tc.format); got != tc.want {
			t.Errorf("gcsContentType(%q): got %q want %q", tc.format, got, tc.want)
		}
	}
}

func TestNewGCS_RejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := NewGCS(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty bucket name")
	}
}
