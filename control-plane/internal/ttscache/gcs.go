package ttscache

import (
	"context"
	"errors"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
)

// GCS is a Google Cloud Storage-backed Cache.
//
// Objects live under <bucket>/tts/<sha256-hex>.<format>. Writes use the
// "object does not exist" precondition so two concurrent misses don't
// overwrite each other — the loser's commit returns PreconditionFailed,
// which we treat as benign (someone else's bytes are equally valid).
//
// Designed for Cloud Functions 2nd gen: the client picks up Application
// Default Credentials automatically from the function's service account.
type GCS struct {
	client *storage.Client
	bucket *storage.BucketHandle
}

// NewGCS connects to the given bucket. Returns an error only if the SDK
// can't initialize; bucket existence is not pre-checked (callers can
// verify via a probe Get if needed).
func NewGCS(ctx context.Context, bucketName string) (*GCS, error) {
	if bucketName == "" {
		return nil, fmt.Errorf("ttscache: empty bucket name")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("ttscache: storage.NewClient: %w", err)
	}
	return &GCS{
		client: client,
		bucket: client.Bucket(bucketName),
	}, nil
}

// Close releases the underlying gRPC connection. Safe to call after Open
// returns — open Writers hold their own context.
func (g *GCS) Close() error {
	return g.client.Close()
}

func (g *GCS) Key(text, voice, format string, speed float64, lang string) string {
	return MakeKey(text, voice, format, speed, lang)
}

func (g *GCS) Get(key, format string) (io.ReadCloser, int64, bool) {
	ctx := context.Background()
	obj := g.bucket.Object(g.objectName(key, format))
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, 0, false
	}
	rc, err := obj.NewReader(ctx)
	if err != nil {
		return nil, 0, false
	}
	return rc, attrs.Size, true
}

func (g *GCS) Open(key, format string) (*Writer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	obj := g.bucket.Object(g.objectName(key, format))
	writer := obj.If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	writer.ContentType = gcsContentType(format)
	return newWriter(&gcsWriter{
		writer: writer,
		cancel: cancel,
	}), nil
}

func (g *GCS) objectName(key, format string) string {
	return "tts/" + key + "." + format
}

func gcsContentType(format string) string {
	switch format {
	case "opus":
		return "audio/opus"
	default:
		return "audio/mpeg"
	}
}

// gcsWriter implements writerImpl over a *storage.Writer.
//
// Writes are streamed to GCS via a resumable upload. Commit calls Close on
// the underlying writer, which finalizes the upload — the object becomes
// visible at that point. Close-without-Commit cancels the upload context
// so GCS never materializes the object.
type gcsWriter struct {
	writer    *storage.Writer
	cancel    context.CancelFunc
	committed bool
	closed    bool
}

func (w *gcsWriter) Write(p []byte) (int, error) {
	return w.writer.Write(p)
}

func (w *gcsWriter) Commit() error {
	if w.committed {
		return nil
	}
	if err := w.writer.Close(); err != nil {
		w.closed = true
		// A concurrent writer beat us to the same key. Their bytes are equally
		// valid; surface success so the handler doesn't log it as an error.
		var apiErr *googleapi.Error
		if errors.As(err, &apiErr) && apiErr.Code == 412 { // Precondition Failed
			w.committed = true
			return nil
		}
		return fmt.Errorf("ttscache: gcs commit: %w", err)
	}
	w.closed = true
	w.committed = true
	return nil
}

func (w *gcsWriter) Close() error {
	if w.committed {
		return nil
	}
	w.cancel()
	if !w.closed {
		_ = w.writer.Close() // best-effort; may error due to canceled context
		w.closed = true
	}
	return nil
}

// Compile-time check that *GCS satisfies the Cache interface.
var _ Cache = (*GCS)(nil)
