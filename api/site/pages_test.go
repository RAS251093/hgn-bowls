package site

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// cloudflare fakes the deploy hook and the deployments API.
type cloudflare struct {
	hookStatus int
	hookCalls  int
	listStatus int
	listBody   string
	gotAuth    string
	gotPath    string
	gotQuery   string
	srv        *httptest.Server
}

func newCloudflare(t *testing.T) *cloudflare {
	t.Helper()
	cf := &cloudflare{hookStatus: 200, listStatus: 200}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /hook/secret-id", func(w http.ResponseWriter, r *http.Request) {
		cf.hookCalls++
		w.WriteHeader(cf.hookStatus)
		io.WriteString(w, `{"success":true}`)
	})
	mux.HandleFunc("GET /client/v4/accounts/{acct}/pages/projects/{proj}/deployments", func(w http.ResponseWriter, r *http.Request) {
		cf.gotAuth = r.Header.Get("Authorization")
		cf.gotPath = r.URL.Path
		cf.gotQuery = r.URL.RawQuery
		w.WriteHeader(cf.listStatus)
		io.WriteString(w, cf.listBody)
	})
	cf.srv = httptest.NewServer(mux)
	t.Cleanup(cf.srv.Close)
	return cf
}

func (cf *cloudflare) pages() *Pages {
	return NewPages(cf.srv.Client(), cf.srv.URL+"/hook/secret-id", "cf-token", "acct1", "hgn-bowls",
		WithBaseURL(cf.srv.URL+"/client/v4"))
}

func TestRebuild(t *testing.T) {
	cf := newCloudflare(t)
	p := cf.pages()
	if err := p.Rebuild(t.Context()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if cf.hookCalls != 1 {
		t.Errorf("hook called %d times, want 1", cf.hookCalls)
	}

	cf.hookStatus = 500
	err := p.Rebuild(t.Context())
	if err == nil {
		t.Fatal("expected an error when the hook fails")
	}
	if got := err.Error(); strings.Contains(got, "secret-id") {
		t.Errorf("error %q leaks the deploy hook URL", got)
	}
}

func TestLatest(t *testing.T) {
	fixture, err := os.ReadFile("testdata/deployments.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		body string
		want *Deployment
	}{
		{name: "live", body: string(fixture), want: &Deployment{
			ID: "d1", Status: StatusLive, Stage: "deploy", CreatedOn: "2026-10-24T09:00:00.000000Z",
			URL: "https://d1.hgn-bowls.pages.dev", Trigger: "deploy_hook",
		}},
		{name: "none yet", body: `{"success":true,"result":[]}`, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf := newCloudflare(t)
			cf.listBody = tt.body
			got, err := cf.pages().Latest(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("latest (-want +got):\n%s", diff)
			}
			if cf.gotAuth != "Bearer cf-token" {
				t.Errorf("Authorization = %q", cf.gotAuth)
			}
			if cf.gotPath != "/client/v4/accounts/acct1/pages/projects/hgn-bowls/deployments" || cf.gotQuery != "env=production" {
				t.Errorf("requested %s?%s", cf.gotPath, cf.gotQuery)
			}
		})
	}
}

func TestStatusOf(t *testing.T) {
	tests := []struct {
		stage, status string
		want          Status
	}{
		{"queued", "active", StatusBuilding},
		{"build", "active", StatusBuilding},
		{"build", "success", StatusBuilding},
		{"deploy", "active", StatusBuilding},
		{"deploy", "success", StatusLive},
		{"build", "failure", StatusFailed},
		{"deploy", "canceled", StatusFailed},
	}
	for _, tt := range tests {
		if got := statusOf(stage{Name: tt.stage, Status: tt.status}); got != tt.want {
			t.Errorf("statusOf(%s, %s) = %s, want %s", tt.stage, tt.status, got, tt.want)
		}
	}
}

func TestLatestErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"server error", 500, `{"success":false}`},
		{"api error", 200, `{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`},
		{"not json", 200, `<html>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf := newCloudflare(t)
			cf.listStatus, cf.listBody = tt.status, tt.body
			if _, err := cf.pages().Latest(t.Context()); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestHandlers(t *testing.T) {
	fixture, _ := os.ReadFile("testdata/deployments.json")
	logger := slog.New(slog.DiscardHandler)
	tests := []struct {
		name       string
		handler    func(p *Pages) http.Handler
		method     string
		breakIt    func(cf *cloudflare)
		wantStatus int
		wantBody   string
	}{
		{name: "latest", handler: func(p *Pages) http.Handler { return p.LatestHandler(logger) }, method: "GET", wantStatus: 200,
			wantBody: `{"deployment":{"id":"d1","status":"live","stage":"deploy","createdOn":"2026-10-24T09:00:00.000000Z","url":"https://d1.hgn-bowls.pages.dev","trigger":"deploy_hook"}}` + "\n"},
		{name: "latest when Cloudflare fails", handler: func(p *Pages) http.Handler { return p.LatestHandler(logger) }, method: "GET",
			breakIt: func(cf *cloudflare) { cf.listStatus = 500 }, wantStatus: 502},
		{name: "rebuild", handler: func(p *Pages) http.Handler { return p.RebuildHandler(logger) }, method: "POST", wantStatus: 200,
			wantBody: `{"rebuild":{"status":"started"}}` + "\n"},
		{name: "rebuild when the hook fails", handler: func(p *Pages) http.Handler { return p.RebuildHandler(logger) }, method: "POST",
			breakIt: func(cf *cloudflare) { cf.hookStatus = 500 }, wantStatus: 502,
			wantBody: `{"error":"the website didn't start updating","rebuild":{"status":"failed"}}` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf := newCloudflare(t)
			cf.listBody = string(fixture)
			if tt.breakIt != nil {
				tt.breakIt(cf)
			}
			rec := httptest.NewRecorder()
			tt.handler(cf.pages()).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tt.method, "/", nil))
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %s\nwant   %s", rec.Body, tt.wantBody)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestRebuildDoesNotLeakHookURLWhenUnreachable(t *testing.T) {
	cf := newCloudflare(t)
	p := cf.pages()
	cf.srv.Close()
	err := p.Rebuild(t.Context())
	if err == nil {
		t.Fatal("expected an error with Cloudflare unreachable")
	}
	if strings.Contains(err.Error(), "secret-id") {
		t.Errorf("error %q leaks the deploy hook URL", err)
	}
}
