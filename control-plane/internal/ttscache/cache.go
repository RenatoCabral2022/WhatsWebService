// Package ttscache stores TTS audio keyed by a content hash.
//
// The Cache interface is deliberately small — Key/Get/Open — so swapping
// the filesystem implementation for object storage (R2, S3, GCS) on the
// upcoming hosting migration is a single constructor change at boot.
package ttscache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
)

// Cache stores opaque audio bytes keyed by a content hash + format.
type Cache interface {
	// Key derives a stable hash from the inputs that materially affect the
	// generated audio. Same inputs → same key.
	Key(text, voice, format string, speed float64, lang string) string

	// Get returns a reader over the cached bytes if the key+format exists,
	// along with their size in bytes, and a found flag. Caller must close
	// the reader.
	Get(key, format string) (rc io.ReadCloser, size int64, ok bool)

	// Open starts a new cache entry. The returned Writer must be either
	// Committed (atomic rename to final path) or Closed without commit
	// (temp file is removed). Close is safe to call after Commit.
	Open(key, format string) (*Writer, error)
}

// MakeKey is the canonical key derivation; exposed so callers can compute keys
// without holding a Cache reference (handy in tests).
func MakeKey(text, voice, format string, speed float64, lang string) string {
	h := sha256.New()
	h.Write([]byte(text))
	h.Write([]byte{0})
	h.Write([]byte(voice))
	h.Write([]byte{0})
	h.Write([]byte(format))
	h.Write([]byte{0})
	h.Write([]byte(strconv.FormatFloat(speed, 'f', 4, 64)))
	h.Write([]byte{0})
	h.Write([]byte(lang))
	return hex.EncodeToString(h.Sum(nil))
}

// Writer is a one-shot write handle returned by Cache.Open. Writes go to a
// temp file; Commit atomically renames to the final path; Close removes the
// temp file unless Commit succeeded.
//
// Typical use:
//
//	w, err := cache.Open(key, "mp3")
//	if err != nil { ... }
//	defer w.Close() // safe even after Commit
//	if _, err := io.Copy(w, src); err != nil { return err }
//	return w.Commit()
type Writer struct {
	// concrete impl is provided by the filesystem backend; this wrapper
	// exists so handler code never depends on FS internals.
	impl writerImpl
}

type writerImpl interface {
	io.Writer
	Commit() error
	Close() error
}

// Write forwards to the underlying file.
func (w *Writer) Write(p []byte) (int, error) { return w.impl.Write(p) }

// Commit atomically promotes the temp file to its final location. After a
// successful Commit, Close becomes a no-op.
func (w *Writer) Commit() error { return w.impl.Commit() }

// Close releases the temp file. If Commit was not called (or failed), the
// temp file is removed so the cache never serves a half-written entry.
func (w *Writer) Close() error { return w.impl.Close() }

// newWriter wraps a backend-specific impl in the exported Writer type.
func newWriter(impl writerImpl) *Writer { return &Writer{impl: impl} }

// ErrCacheNotFound is the sentinel for a missing key. Get returns ok=false
// rather than this error; backends that need to surface "not found" through
// an error channel can use it.
var ErrCacheNotFound = errors.New("ttscache: not found")
