// Package site talks to Cloudflare Pages, which builds and serves the
// public website: it starts rebuilds and reports how the latest one went.
package site

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

const defaultBaseURL = "https://api.cloudflare.com/client/v4"

// Status is how a site build is going, in the editor's terms.
type Status string

const (
	StatusBuilding Status = "building"
	StatusLive     Status = "live"
	StatusFailed   Status = "failed"
)

// Deployment is the latest production build of the site.
type Deployment struct {
	ID        string `json:"id"`
	Status    Status `json:"status"`
	Stage     string `json:"stage"`
	CreatedOn string `json:"createdOn"`
	URL       string `json:"url"`
	Trigger   string `json:"trigger"`
}

// Pages starts and inspects builds of one Cloudflare Pages project.
type Pages struct {
	client    *http.Client
	hookURL   string
	apiToken  string
	accountID string
	project   string
	baseURL   string
}

// Option changes how Pages is built.
type Option func(*Pages)

// WithBaseURL points at a different Cloudflare API (tests use a fake).
func WithBaseURL(u string) Option { return func(p *Pages) { p.baseURL = u } }

// NewPages returns a Pages client. hookURL is the project's deploy hook;
// apiToken needs read access to the project's deployments. client should
// have a timeout.
func NewPages(client *http.Client, hookURL, apiToken, accountID, project string, opts ...Option) *Pages {
	p := &Pages{client: client, hookURL: hookURL, apiToken: apiToken, accountID: accountID, project: project, baseURL: defaultBaseURL}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Rebuild asks Cloudflare to build the production site. Cloudflare queues
// builds itself, so calling it while one runs is fine.
func (p *Pages) Rebuild(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.hookURL, nil)
	if err != nil {
		return fmt.Errorf("rebuild: bad deploy hook URL")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		// The hook URL is a secret, and url.Error would print it.
		if uerr, ok := errors.AsType[*url.Error](err); ok {
			err = uerr.Err
		}
		return fmt.Errorf("rebuild: call deploy hook: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("rebuild: deploy hook answered %s", resp.Status)
	}
	return nil
}

// stage is the step a Cloudflare build has reached.
type stage struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type deploymentsResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result []struct {
		ID          string `json:"id"`
		URL         string `json:"url"`
		CreatedOn   string `json:"created_on"`
		LatestStage stage  `json:"latest_stage"`
		Trigger     struct {
			Type string `json:"type"`
		} `json:"deployment_trigger"`
	} `json:"result"`
}

// Latest returns the newest production deployment, or nil if there are none.
func (p *Pages) Latest(ctx context.Context) (*Deployment, error) {
	u := fmt.Sprintf("%s/accounts/%s/pages/projects/%s/deployments?env=production",
		p.baseURL, url.PathEscape(p.accountID), url.PathEscape(p.project))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("latest deployment: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("latest deployment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("latest deployment: Cloudflare answered %s", resp.Status)
	}

	var body deploymentsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("latest deployment: decode: %w", err)
	}
	if !body.Success {
		msg := "unknown error"
		if len(body.Errors) > 0 {
			msg = fmt.Sprintf("%d %s", body.Errors[0].Code, body.Errors[0].Message)
		}
		return nil, fmt.Errorf("latest deployment: Cloudflare error: %s", msg)
	}
	if len(body.Result) == 0 {
		return nil, nil
	}
	d := body.Result[0]
	return &Deployment{
		ID:        d.ID,
		Status:    statusOf(d.LatestStage),
		Stage:     d.LatestStage.Name,
		CreatedOn: d.CreatedOn,
		URL:       d.URL,
		Trigger:   d.Trigger.Type,
	}, nil
}

// statusOf reads a build's state from its latest stage. A build is only
// live once the final deploy stage has succeeded.
func statusOf(s stage) Status {
	switch {
	case s.Status == "failure" || s.Status == "canceled":
		return StatusFailed
	case s.Name == "deploy" && s.Status == "success":
		return StatusLive
	default:
		return StatusBuilding
	}
}

// LatestHandler serves GET /api/admin/deploys/latest.
func (p *Pages) LatestHandler(logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := p.Latest(r.Context())
		if err != nil {
			logger.Error("reading latest deployment", "err", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "couldn't check the website's status"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]*Deployment{"deployment": d})
	})
}

// RebuildHandler serves POST /api/admin/rebuild.
func (p *Pages) RebuildHandler(logger *slog.Logger) http.Handler {
	type rebuild struct {
		Status string `json:"status"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := p.Rebuild(context.WithoutCancel(r.Context())); err != nil {
			logger.Error("starting site rebuild", "err", err)
			writeJSON(w, http.StatusBadGateway, struct {
				Error   string  `json:"error"`
				Rebuild rebuild `json:"rebuild"`
			}{"the website didn't start updating", rebuild{"failed"}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]rebuild{"rebuild": {"started"}})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
