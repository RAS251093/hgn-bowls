// Package config reads the API's settings from its environment.
package config

import (
	"cmp"
	"fmt"
	"strings"
)

// Config holds the settings the API reads from its environment at startup.
type Config struct {
	Port               string
	GoogleCloudProject string // optional; used to link log lines to traces

	CalendarID     string
	CalendarAPIKey string
	AllowedOrigins []string

	TursoURL   string
	TursoToken string // not needed for a local file: database

	Auth0Domain   string
	Auth0Audience string
	BuildToken    string

	Bucket string

	PagesDeployHook     string
	CloudflareAPIToken  string
	CloudflareAccountID string
	PagesProject        string

	// Optional: inviting admins is off until all four are set, so the API
	// can be deployed before the Auth0 management app and its secret exist.
	Auth0MgmtClientID     string // machine-to-machine app for the Management API
	Auth0MgmtClientSecret string
	Auth0AdminRoleID      string // the club-admin role
	Auth0AdminClientID    string // the admin area's own app, for password emails
}

// AdminManagementMissing names the unset variables that inviting admins
// needs. It's empty when inviting admins is on.
func (c Config) AdminManagementMissing() []string {
	var missing []string
	for _, v := range []struct{ name, value string }{
		{"AUTH0_MGMT_CLIENT_ID", c.Auth0MgmtClientID},
		{"AUTH0_MGMT_CLIENT_SECRET", c.Auth0MgmtClientSecret},
		{"AUTH0_ADMIN_ROLE_ID", c.Auth0AdminRoleID},
		{"AUTH0_ADMIN_CLIENT_ID", c.Auth0AdminClientID},
	} {
		if v.value == "" {
			missing = append(missing, v.name)
		}
	}
	return missing
}

// required lists the variables the API cannot start without.
var required = []string{
	"CALENDAR_ID", "CALENDAR_API_KEY", "ALLOWED_ORIGINS",
	"TURSO_URL",
	"AUTH0_DOMAIN", "AUTH0_AUDIENCE", "BUILD_TOKEN",
	"BUCKET",
	"PAGES_DEPLOY_HOOK", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID", "PAGES_PROJECT",
}

// Load reads the API's settings using getenv (os.Getenv in production)
// and reports every required value that's missing. Error messages name
// variables, never their values.
func Load(getenv func(string) string) (Config, error) {
	var missing []string
	for _, name := range required {
		if getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if url := getenv("TURSO_URL"); url != "" && !strings.HasPrefix(url, "file:") && getenv("TURSO_TOKEN") == "" {
		missing = append(missing, "TURSO_TOKEN")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	return Config{
		Port:                cmp.Or(getenv("PORT"), "8080"),
		GoogleCloudProject:  getenv("GOOGLE_CLOUD_PROJECT"),
		CalendarID:          getenv("CALENDAR_ID"),
		CalendarAPIKey:      getenv("CALENDAR_API_KEY"),
		AllowedOrigins:      splitList(getenv("ALLOWED_ORIGINS")),
		TursoURL:            getenv("TURSO_URL"),
		TursoToken:          getenv("TURSO_TOKEN"),
		Auth0Domain:         getenv("AUTH0_DOMAIN"),
		Auth0Audience:       getenv("AUTH0_AUDIENCE"),
		BuildToken:          getenv("BUILD_TOKEN"),
		Bucket:              getenv("BUCKET"),
		PagesDeployHook:     getenv("PAGES_DEPLOY_HOOK"),
		CloudflareAPIToken:  getenv("CLOUDFLARE_API_TOKEN"),
		CloudflareAccountID: getenv("CLOUDFLARE_ACCOUNT_ID"),
		PagesProject:        getenv("PAGES_PROJECT"),

		Auth0MgmtClientID:     getenv("AUTH0_MGMT_CLIENT_ID"),
		Auth0MgmtClientSecret: getenv("AUTH0_MGMT_CLIENT_SECRET"),
		Auth0AdminRoleID:      getenv("AUTH0_ADMIN_ROLE_ID"),
		Auth0AdminClientID:    getenv("AUTH0_ADMIN_CLIENT_ID"),
	}, nil
}

func splitList(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
