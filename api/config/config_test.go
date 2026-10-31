package config

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad(t *testing.T) {
	full := map[string]string{
		"CALENDAR_ID":           "club@group.calendar.google.com",
		"CALENDAR_API_KEY":      "key",
		"ALLOWED_ORIGINS":       "https://example.org, http://localhost:4321",
		"TURSO_URL":             "libsql://club.turso.io",
		"TURSO_TOKEN":           "turso-token",
		"AUTH0_DOMAIN":          "club.eu.auth0.com",
		"AUTH0_AUDIENCE":        "https://api.example.org",
		"BUILD_TOKEN":           "build-token",
		"BUCKET":                "club-images",
		"PAGES_DEPLOY_HOOK":     "https://api.cloudflare.com/hook",
		"CLOUDFLARE_API_TOKEN":  "cf-token",
		"CLOUDFLARE_ACCOUNT_ID": "cf-account",
		"PAGES_PROJECT":         "hgn-bowls",
	}
	with := func(changes map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range full {
			m[k] = v
		}
		for k, v := range changes {
			m[k] = v
		}
		return m
	}
	without := func(k string) map[string]string { return with(map[string]string{k: ""}) }

	want := Config{
		Port:                "8080",
		CalendarID:          "club@group.calendar.google.com",
		CalendarAPIKey:      "key",
		AllowedOrigins:      []string{"https://example.org", "http://localhost:4321"},
		TursoURL:            "libsql://club.turso.io",
		TursoToken:          "turso-token",
		Auth0Domain:         "club.eu.auth0.com",
		Auth0Audience:       "https://api.example.org",
		BuildToken:          "build-token",
		Bucket:              "club-images",
		PagesDeployHook:     "https://api.cloudflare.com/hook",
		CloudflareAPIToken:  "cf-token",
		CloudflareAccountID: "cf-account",
		PagesProject:        "hgn-bowls",
	}
	withProject := want
	withProject.Port = "9090"
	withProject.GoogleCloudProject = "hgn-bowls"
	localDB := want
	localDB.TursoURL = "file:local.db"
	localDB.TursoToken = ""

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr []string
	}{
		{name: "all required set, optional defaults", env: full, want: want},
		{name: "optional values read", env: with(map[string]string{"PORT": "9090", "GOOGLE_CLOUD_PROJECT": "hgn-bowls"}), want: withProject},
		{name: "local database needs no token", env: with(map[string]string{"TURSO_URL": "file:local.db", "TURSO_TOKEN": ""}), want: localDB},
		{name: "remote database needs a token", env: without("TURSO_TOKEN"), wantErr: []string{"TURSO_TOKEN"}},
		{name: "nothing set names everything", env: map[string]string{}, wantErr: []string{
			"CALENDAR_ID", "CALENDAR_API_KEY", "ALLOWED_ORIGINS", "TURSO_URL", "AUTH0_DOMAIN", "AUTH0_AUDIENCE",
			"BUILD_TOKEN", "BUCKET", "PAGES_DEPLOY_HOOK", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID", "PAGES_PROJECT",
		}},
	}
	for _, name := range []string{
		"CALENDAR_ID", "CALENDAR_API_KEY", "ALLOWED_ORIGINS", "TURSO_URL", "AUTH0_DOMAIN", "AUTH0_AUDIENCE",
		"BUILD_TOKEN", "BUCKET", "PAGES_DEPLOY_HOOK", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID", "PAGES_PROJECT",
	} {
		tests = append(tests, struct {
			name    string
			env     map[string]string
			want    Config
			wantErr []string
		}{name: "missing " + name, env: without(name), wantErr: []string{name}})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(env(tt.env))
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				for _, name := range tt.wantErr {
					if !strings.Contains(err.Error(), name) {
						t.Errorf("error %q does not mention %s", err, name)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("config mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadDoesNotLeakSecretsInErrors(t *testing.T) {
	_, err := Load(env(map[string]string{"TURSO_URL": "libsql://x", "BUILD_TOKEN": "s3cret"}))
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Errorf("error %q contains a secret value", err)
	}
}

func TestAdminManagementIsOptional(t *testing.T) {
	base := map[string]string{
		"CALENDAR_ID": "c", "CALENDAR_API_KEY": "k", "ALLOWED_ORIGINS": "https://example.org",
		"TURSO_URL": "file:local.db", "AUTH0_DOMAIN": "club.eu.auth0.com", "AUTH0_AUDIENCE": "https://api.example.org",
		"BUILD_TOKEN": "b", "BUCKET": "img", "PAGES_DEPLOY_HOOK": "https://hook", "CLOUDFLARE_API_TOKEN": "t",
		"CLOUDFLARE_ACCOUNT_ID": "a", "PAGES_PROJECT": "p",
	}
	admin := map[string]string{
		"AUTH0_MGMT_CLIENT_ID":     "mgmt-id",
		"AUTH0_MGMT_CLIENT_SECRET": "mgmt-secret",
		"AUTH0_ADMIN_ROLE_ID":      "rol_admin",
		"AUTH0_ADMIN_CLIENT_ID":    "spa-id",
	}
	allAdmin := []string{"AUTH0_MGMT_CLIENT_ID", "AUTH0_MGMT_CLIENT_SECRET", "AUTH0_ADMIN_ROLE_ID", "AUTH0_ADMIN_CLIENT_ID"}

	t.Run("none set: starts, with inviting off", func(t *testing.T) {
		got, err := Load(env(base))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if diff := cmp.Diff(allAdmin, got.AdminManagementMissing()); diff != "" {
			t.Errorf("missing (-want +got):\n%s", diff)
		}
	})

	t.Run("all set: inviting on", func(t *testing.T) {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range admin {
			m[k] = v
		}
		got, err := Load(env(m))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if missing := got.AdminManagementMissing(); len(missing) != 0 {
			t.Errorf("missing = %v, want none", missing)
		}
		want := [4]string{"mgmt-id", "mgmt-secret", "rol_admin", "spa-id"}
		if g := [4]string{got.Auth0MgmtClientID, got.Auth0MgmtClientSecret, got.Auth0AdminRoleID, got.Auth0AdminClientID}; g != want {
			t.Errorf("admin settings = %v, want %v", g, want)
		}
	})

	t.Run("some set: still off, naming the rest", func(t *testing.T) {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		m["AUTH0_MGMT_CLIENT_ID"] = "mgmt-id"
		m["AUTH0_ADMIN_ROLE_ID"] = "rol_admin"
		got, err := Load(env(m))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"AUTH0_MGMT_CLIENT_SECRET", "AUTH0_ADMIN_CLIENT_ID"}
		if diff := cmp.Diff(want, got.AdminManagementMissing()); diff != "" {
			t.Errorf("missing (-want +got):\n%s", diff)
		}
	})
}

func TestCustomDomainIsOptional(t *testing.T) {
	base := map[string]string{
		"CALENDAR_ID": "c", "CALENDAR_API_KEY": "k", "ALLOWED_ORIGINS": "https://example.org",
		"TURSO_URL": "file:local.db", "AUTH0_DOMAIN": "club.eu.auth0.com", "AUTH0_AUDIENCE": "https://api.example.org",
		"BUILD_TOKEN": "b", "BUCKET": "img", "PAGES_DEPLOY_HOOK": "https://hook", "CLOUDFLARE_API_TOKEN": "t",
		"CLOUDFLARE_ACCOUNT_ID": "a", "PAGES_PROJECT": "p",
	}
	got, err := Load(func(k string) string { return base[k] })
	if err != nil || got.Auth0CustomDomain != "" {
		t.Fatalf("without it: %q, %v; want empty and no error", got.Auth0CustomDomain, err)
	}
	base["AUTH0_CUSTOM_DOMAIN"] = "auth.example.org"
	got, err = Load(func(k string) string { return base[k] })
	if err != nil || got.Auth0CustomDomain != "auth.example.org" {
		t.Errorf("with it: %q, %v", got.Auth0CustomDomain, err)
	}
}
