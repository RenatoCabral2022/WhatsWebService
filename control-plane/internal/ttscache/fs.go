package ttscache

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FS is a filesystem-backed Cache. Entries live at <dir>/<key>.<format>.
//
// Hosting-portable: works on any host that gives the process a writable
// directory. On serverless / ephemeral-disk hosts, swap this for an
// object-storage implementation behind the same Cache interface.
type FS struct {
	dir string
}

// NewFS ensures dir exists (creating it with 0o755 perms if needed) and
// returns the cache. Errors are returned only for unrecoverable cases
// (e.g. dir is a file, or no write permission).
func NewFS(dir string) (*FS, error) {
	if dir == "" {
		return nil, fmt.Errorf("ttscache: empty cache dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("ttscache: mkdir %s: %w", dir, err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("ttscache: stat %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("ttscache: %s is not a directory", dir)
	}
	return &FS{dir: dir}, nil
}

// Key delegates to MakeKey — same canonical derivation, exposed via Cache.
func (f *FS) Key(text, voice, format string, speed float64, lang string) string {
	return MakeKey(text, voice, format, speed, lang)
}

// Get opens the cached file if it exists. Caller closes the reader.
func (f *FS) Get(key, format string) (io.ReadCloser, int64, bool) {
	path := f.finalPath(key, format)
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, false
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, false
	}
	return file, info.Size(), true
}

// Open starts a new cache entry. Writes go to a randomly-named temp file in
// the same directory so the final rename is atomic on the same filesystem.
func (f *FS) Open(key, format string) (*Writer, error) {
	finalPath := f.finalPath(key, format)
	tmpSuffix, err := randomSuffix()
	if err != nil {
		return nil, fmt.Errorf("ttscache: random suffix: %w", err)
	}
	tmpPath := finalPath + ".tmp." + tmpSuffix

	file, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("ttscache: create temp %s: %w", tmpPath, err)
	}

	return newWriter(&fsWriter{
		file:      file,
		tmpPath:   tmpPath,
		finalPath: finalPath,
	}), nil
}

func (f *FS) finalPath(key, format string) string {
	return filepath.Join(f.dir, key+"."+format)
}

// fsWriter implements writerImpl over a real *os.File.
type fsWriter struct {
	file      *os.File
	tmpPath   string
	finalPath string
	committed bool
	closed    bool
}

func (w *fsWriter) Write(p []byte) (int, error) {
	return w.file.Write(p)
}

// Commit fsyncs and atomically renames the temp file to its final path.
// After a successful Commit, Close is a no-op (the file is already gone
// from the temp path).
func (w *fsWriter) Commit() error {
	if w.committed {
		return nil
	}
	// Best-effort fsync; proceed with the rename either way.
	_ = w.file.Sync()
	if err := w.file.Close(); err != nil {
		_ = os.Remove(w.tmpPath)
		return fmt.Errorf("ttscache: close temp before rename: %w", err)
	}
	w.closed = true
	if err := os.Rename(w.tmpPath, w.finalPath); err != nil {
		_ = os.Remove(w.tmpPath)
		return fmt.Errorf("ttscache: rename %s -> %s: %w", w.tmpPath, w.finalPath, err)
	}
	w.committed = true
	return nil
}

// Close releases the temp file. If Commit succeeded earlier, this is a no-op.
// Otherwise the temp file is removed so we never leak half-written entries.
func (w *fsWriter) Close() error {
	if w.committed {
		return nil
	}
	var closeErr error
	if !w.closed {
		closeErr = w.file.Close()
		w.closed = true
	}
	_ = os.Remove(w.tmpPath)
	return closeErr
}

func randomSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Compile-time check that *FS satisfies the Cache interface.
var _ Cache = (*FS)(nil)
