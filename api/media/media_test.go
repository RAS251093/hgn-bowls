package media

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/go-cmp/cmp"
)

// keySigner signs for real with a throwaway key, so the tests check that the
// options we pass make a valid V4 URL, not just that we passed them.
type keySigner struct {
	t    *testing.T
	key  []byte
	last *storage.SignedURLOptions
}

func newKeySigner(t *testing.T) *keySigner {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return &keySigner{t: t, key: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})}
}

func (s *keySigner) SignedURL(object string, opts *storage.SignedURLOptions) (string, error) {
	s.last = opts
	o := *opts
	o.GoogleAccessID = "api-runtime@example.iam.gserviceaccount.com"
	o.PrivateKey = s.key
	return storage.SignedURL("club-images", object, &o)
}

// newTestBucket uses the real clock: the storage package dates signatures
// with it, so a fake one would skew the expiry.
func newTestBucket(t *testing.T) (*Bucket, *keySigner) {
	s := newKeySigner(t)
	return NewBucket(s,
		WithIDs(func() string { return "01JABCDEF" }),
	), s
}

func TestUploadHandler(t *testing.T) {
	b, signer := newTestBucket(t)
	h := b.UploadHandler(slog.New(slog.DiscardHandler))
	month := time.Now().UTC().Format("2006/01")

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantKey    string
		wantField  string
	}{
		{name: "jpeg", body: `{"contentType":"image/jpeg","size":2048}`, wantStatus: 200, wantKey: "posts/" + month + "/01JABCDEF.jpg"},
		{name: "png", body: `{"contentType":"image/png","size":1}`, wantStatus: 200, wantKey: "posts/" + month + "/01JABCDEF.png"},
		{name: "webp at the limit", body: `{"contentType":"image/webp","size":10485760}`, wantStatus: 200, wantKey: "posts/" + month + "/01JABCDEF.webp"},
		{name: "too big", body: `{"contentType":"image/jpeg","size":10485761}`, wantStatus: 422, wantField: "size"},
		{name: "empty file", body: `{"contentType":"image/jpeg","size":0}`, wantStatus: 422, wantField: "size"},
		{name: "svg refused", body: `{"contentType":"image/svg+xml","size":10}`, wantStatus: 422, wantField: "contentType"},
		{name: "not json", body: `nope`, wantStatus: 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), "POST", "/api/admin/uploads", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantField != "" {
				var got struct{ Fields map[string]string }
				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if got.Fields[tt.wantField] == "" {
					t.Errorf("422 does not name %s: %+v", tt.wantField, got)
				}
			}
			if tt.wantStatus != 200 {
				return
			}

			var got Upload
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.Key != tt.wantKey {
				t.Errorf("key = %q, want %q", got.Key, tt.wantKey)
			}
			u, err := url.Parse(got.UploadURL)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(u.Path, "/club-images/"+tt.wantKey) {
				t.Errorf("upload URL path %q is not for the key", u.Path)
			}
			q := u.Query()
			if exp := q.Get("X-Goog-Expires"); exp != "300" && exp != "299" {
				t.Errorf("expires = %s, want 300 seconds", q.Get("X-Goog-Expires"))
			}
			for _, h := range []string{"content-type", "x-goog-content-length-range"} {
				if !strings.Contains(q.Get("X-Goog-SignedHeaders"), h) {
					t.Errorf("signed headers %q do not lock %s", q.Get("X-Goog-SignedHeaders"), h)
				}
			}
			if signer.last.Method != "PUT" || signer.last.Scheme != storage.SigningSchemeV4 {
				t.Errorf("signed %s with scheme %v, want a V4 PUT", signer.last.Method, signer.last.Scheme)
			}

			var sent uploadRequest
			if err := json.Unmarshal([]byte(tt.body), &sent); err != nil {
				t.Fatal(err)
			}
			wantHeaders := map[string]string{
				"Content-Type":                sent.ContentType,
				"x-goog-content-length-range": "0," + strconv.FormatInt(sent.Size, 10),
			}
			if diff := cmp.Diff(wantHeaders, got.Headers); diff != "" {
				t.Errorf("headers (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSignedReadURL(t *testing.T) {
	b, signer := newTestBucket(t)
	got, err := b.SignedReadURL(t.Context(), "posts/2026/10/x.jpg")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	if exp := u.Query().Get("X-Goog-Expires"); (exp != "3600" && exp != "3599") || signer.last.Method != "GET" {
		t.Errorf("read URL %s: want a GET valid for an hour", got)
	}
	if _, err := b.SignedReadURL(t.Context(), "../secret"); err == nil {
		t.Error("signed a URL for a key outside posts/")
	}
}

func TestUploadHandlerRejectsHugeBody(t *testing.T) {
	b, _ := newTestBucket(t)
	h := b.UploadHandler(slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(t.Context(), "POST", "/api/admin/uploads", strings.NewReader(strings.Repeat(" ", 1<<20)+"{}"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
