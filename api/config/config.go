package config

import (
	"cmp"
	"fmt"
	"strings"
)

type Config struct {
	Port           string
	CalendarID     string
	CalendarAPIKey string
	AllowedOrigins []string
}

var required = []string{"CALENDAR_ID", "CALENDAR_API_KEY", "ALLOWED_ORIGINS"}

func Load(getenv func(string) string) (Config, error) {
	var missing []string
	for _, name := range required {
		if getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	return Config{
		Port:           cmp.Or(getenv("PORT"), "8080"),
		CalendarID:     getenv("CALENDAR_ID"),
		CalendarAPIKey: getenv("CALENDAR_API_KEY"),
		AllowedOrigins: splitList(getenv("ALLOWED_ORIGINS")),
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
