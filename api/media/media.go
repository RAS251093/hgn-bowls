// Package media hands out short-lived Cloud Storage links: signed PUT URLs
// so admins upload images straight to the bucket, and signed GET URLs so
// the site build can download them.
package media

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/oklog/ulid/v2"
)

const (
	maxUpload     = 10 << 20 // 10 MB
	uploadExpiry  = 5 * time.Minute
	readExpiry    = time.Hour
	keyPrefix     = "posts/"
	maxBodyBuffer = 1 << 10
)

// extensions lists the image types admins may upload.
var extensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// urlSigner signs URLs for objects in one bucket (*storage.BucketHandle in production).
type urlSigner interface {
	SignedURL(object string, opts *storage.SignedURLOptions) (string, error)
}

// Bucket signs upload and download URLs for post images.
type Bucket struct {
	signer urlSigner
	now    func() time.Time
	newID  func() string
}

// Option changes how a Bucket is built.
type Option func(*Bucket)

// WithIDs sets how the unique part of new object names is made.
func WithIDs(newID func() string) Option { return func(b *Bucket) { b.newID = newID } }

// NewBucket returns a Bucket signing with signer. In production pass
// client.Bucket(name): it signs as the Cloud Run service account through
// the IAM Credentials API, so no key file is needed.
func NewBucket(signer urlSigner, opts ...Option) *Bucket {
	b := &Bucket{signer: signer, now: time.Now, newID: func() string { return ulid.Make().String() }}
	for _, o := range opts {
		o(b)
	}
	return b
}

// SignedReadURL returns a link to download key, valid for an hour.
func (b *Bucket) SignedReadURL(ctx context.Context, key string) (string, error) {
	if !strings.HasPrefix(key, keyPrefix) || strings.Contains(key, "..") {
		return "", fmt.Errorf("sign read URL: key %q is not a post image", key)
	}
	u, err := b.signer.SignedURL(key, &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  http.MethodGet,
		Expires: b.now().Add(readExpiry),
	})
	if err != nil {
		return "", fmt.Errorf("sign read URL for %s: %w", key, err)
	}
	return u, nil
}

type uploadRequest struct {
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

// Upload tells the browser where and how to PUT an image. The browser must
// send every header in Headers, or Cloud Storage rejects the upload.
type Upload struct {
	Key       string            `json:"key"`
	UploadURL string            `json:"uploadURL"`
	Headers   map[string]string `json:"headers"`
}

// UploadHandler serves POST /api/admin/uploads. The server picks the object
// name; the URL is locked to it, the content type and the declared size.
func (b *Bucket) UploadHandler(logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req uploadRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBuffer))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "send {contentType, size} as JSON")
			return
		}

		fields := map[string]string{}
		ext, ok := extensions[req.ContentType]
		if !ok {
			fields["contentType"] = "Upload a JPEG, PNG or WebP image"
		}
		if req.Size < 1 || req.Size > maxUpload {
			fields["size"] = "Images must be 10 MB or smaller"
		}
		if len(fields) > 0 {
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "that file can't be uploaded", "fields": fields})
			return
		}

		now := b.now().UTC()
		key := fmt.Sprintf("%s%04d/%02d/%s.%s", keyPrefix, now.Year(), now.Month(), b.newID(), ext)
		lengthRange := "0," + strconv.FormatInt(req.Size, 10)
		u, err := b.signer.SignedURL(key, &storage.SignedURLOptions{
			Scheme:      storage.SigningSchemeV4,
			Method:      http.MethodPut,
			Expires:     now.Add(uploadExpiry),
			ContentType: req.ContentType,
			Headers:     []string{"x-goog-content-length-range:" + lengthRange},
		})
		if err != nil {
			logger.Error("signing upload URL", "err", err, "key", key)
			writeError(w, http.StatusInternalServerError, "uploads aren't working right now")
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, Upload{
			Key:       key,
			UploadURL: u,
			Headers: map[string]string{
				"Content-Type":                req.ContentType,
				"x-goog-content-length-range": lengthRange,
			},
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": msg})
}
