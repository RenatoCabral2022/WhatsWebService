package ttscache

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeKey_DeterministicAndDistinct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		a, b    [5]any
		wantEq  bool
		message string
	}{
		{
			name:    "same inputs same key",
			a:       [5]any{"hello", "nova", "mp3", 1.0, "en"},
			b:       [5]any{"hello", "nova", "mp3", 1.0, "en"},
			wantEq:  true,
			message: "identical inputs must produce identical keys",
		},
		{
			name:    "different text different key",
			a:       [5]any{"hello", "nova", "mp3", 1.0, "en"},
			b:       [5]any{"hello!", "nova", "mp3", 1.0, "en"},
			wantEq:  false,
			message: "text change must alter the key",
		},
		{
			name:    "different voice different key",
			a:       [5]any{"hello", "nova", "mp3", 1.0, "en"},
			b:       [5]any{"hello", "shimmer", "mp3", 1.0, "en"},
			wantEq:  false,
			message: "voice change must alter the key",
		},
		{
			name:    "different format different key",
			a:       [5]any{"hello", "nova", "mp3", 1.0, "en"},
			b:       [5]any{"hello", "nova", "opus", 1.0, "en"},
			wantEq:  false,
			message: "format change must alter the key",
		},
		{
			name:    "different speed different key",
			a:       [5]any{"hello", "nova", "mp3", 1.0, "en"},
			b:       [5]any{"hello", "nova", "mp3", 1.25, "en"},
			wantEq:  false,
			message: "speed change must alter the key",
		},
		{
			name:    "different language different key",
			a:       [5]any{"hello", "nova", "mp3", 1.0, "en"},
			b:       [5]any{"hello", "nova", "mp3", 1.0, "pt"},
			wantEq:  false,
			message: "lang change must alter the key",
		},
		{
			name:    "delimiter prevents field collision",
			a:       [5]any{"ab", "cd", "mp3", 1.0, "en"},
			b:       [5]any{"abcd", "", "mp3", 1.0, "en"},
			wantEq:  false,
			message: "concatenation without a delimiter would collide; null-byte separator prevents it",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ka := MakeKey(tc.a[0].(string), tc.a[1].(string), tc.a[2].(string), tc.a[3].(float64), tc.a[4].(string))
			kb := MakeKey(tc.b[0].(string), tc.b[1].(string), tc.b[2].(string), tc.b[3].(float64), tc.b[4].(string))
			if (ka == kb) != tc.wantEq {
				t.Errorf("%s: got eq=%t (ka=%q kb=%q)", tc.message, ka == kb, ka, kb)
			}
			if len(ka) != 64 {
				t.Errorf("expected 64-char hex key, got %d chars", len(ka))
			}
		})
	}
}

func TestFS_OpenCommitGet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}

	key := c.Key("hello world", "nova", "mp3", 1.0, "en")
	w, err := c.Open(key, "mp3")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	want := []byte("fake-audio-bytes")
	if _, err := w.Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("Close after Commit should be no-op, got %v", err)
	}

	rc, size, ok := c.Get(key, "mp3")
	if !ok {
		t.Fatalf("Get: not found after Commit")
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
}

func TestFS_AbortRemovesTempAndLeavesNoEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}

	key := c.Key("dropped", "nova", "mp3", 1.0, "en")
	w, err := c.Open(key, "mp3")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := w.Write([]byte("half-data")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Close without Commit — simulates client disconnect mid-stream.
	if err := w.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	if _, _, ok := c.Get(key, "mp3"); ok {
		t.Fatalf("Get returned ok after abort — should be miss")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("leftover temp file after abort: %s", e.Name())
		}
	}
}

func TestFS_GetMiss(t *testing.T) {
	t.Parallel()
	c, err := NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	if rc, _, ok := c.Get("no-such-key", "mp3"); ok {
		_ = rc.Close()
		t.Fatal("expected miss")
	}
}

func TestNewFS_RejectsFile(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(tmp, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup write: %v", err)
	}
	if _, err := NewFS(tmp); err == nil {
		t.Fatal("expected error for non-directory path")
	}
}

func TestNewFS_RejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := NewFS(""); err == nil {
		t.Fatal("expected error for empty dir")
	}
}
