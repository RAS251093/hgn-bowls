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
		"CALENDAR_ID":      "club@group.calendar.google.com",
		"CALENDAR_API_KEY": "key",
		"ALLOWED_ORIGINS":  "https://example.org, http://localhost:4321",
	}
	without := func(k string) map[string]string {
		m := map[string]string{}
		for kk, v := range full {
			if kk != k {
				m[kk] = v
			}
		}
		return m
	}

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr []string
	}{
		{
			name: "all env vars, port defaults",
			env:  full,
			want: Config{
				Port:           "8080",
				CalendarID:     "club@group.calendar.google.com",
				CalendarAPIKey: "key",
				AllowedOrigins: []string{"https://example.org", "http://localhost:4321"},
			},
		},
		{name: "missing calendar id", env: without("CALENDAR_ID"), wantErr: []string{"CALENDAR_ID"}},
		{name: "missing api key", env: without("CALENDAR_API_KEY"), wantErr: []string{"CALENDAR_API_KEY"}},
		{name: "missing origins", env: without("ALLOWED_ORIGINS"), wantErr: []string{"ALLOWED_ORIGINS"}},
		{name: "nothing set should name everything", env: map[string]string{}, wantErr: []string{"CALENDAR_ID", "CALENDAR_API_KEY", "ALLOWED_ORIGINS"}},
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
