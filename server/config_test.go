package main

import (
	"log/slog"
	"maps"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	valid := map[string]string{
		"DATABASE_URL":         "postgres://u:p@db/orbit",
		"ANISETTE_UPSTREAM":    "http://anisette-v3:6969",
		"PUBLIC_URL":           "https://orbit.example.com",
		"JWT_SECRET":           strings.Repeat("k", 32),
		"GITHUB_CLIENT_ID":     "id",
		"GITHUB_CLIENT_SECRET": "secret",
	}
	tests := []struct {
		name    string
		set     map[string]string
		wantErr string
	}{
		{"defaults", nil, ""},
		{"localhost over http", map[string]string{"PUBLIC_URL": "http://localhost:8080"}, ""},
		{"smtp only", map[string]string{"GITHUB_CLIENT_ID": "", "GITHUB_CLIENT_SECRET": "", "SMTP_HOST": "smtp.example.com", "SMTP_FROM": "a@example.com"}, ""},
		{"no database", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL"},
		{"example database password", map[string]string{"DATABASE_URL": "password='change-me-openssl'"}, "example secret"},
		{"upstream without scheme", map[string]string{"ANISETTE_UPSTREAM": "anisette:6969"}, "ANISETTE_UPSTREAM"},
		{"bad level", map[string]string{"LOG_LEVEL": "loud"}, "LOG_LEVEL"},
		{"public http", map[string]string{"PUBLIC_URL": "http://orbit.example.com"}, "https"},
		{"short secret", map[string]string{"JWT_SECRET": "short"}, "JWT_SECRET"},
		{"example secret", map[string]string{"JWT_SECRET": "change-me-openssl-rand-base64-48"}, "JWT_SECRET"},
		{"bad duration", map[string]string{"ACCESS_TOKEN_TTL": "15"}, "ACCESS_TOKEN_TTL"},
		{"bad device cap", map[string]string{"MAX_DEVICES_PER_USER": "0"}, "MAX_DEVICES_PER_USER"},
		{"half a provider", map[string]string{"GOOGLE_CLIENT_ID": "id"}, "GOOGLE_CLIENT_SECRET"},
		{"smtp without sender", map[string]string{"SMTP_HOST": "smtp.example.com"}, "SMTP_FROM"},
		{"half a vapid pair", map[string]string{"VAPID_PUBLIC_KEY": "k"}, "VAPID_PRIVATE_KEY"},
		{"vapid subject", map[string]string{"VAPID_PUBLIC_KEY": "k", "VAPID_PRIVATE_KEY": "k", "VAPID_SUBJECT": "ops"}, "VAPID_SUBJECT"},
		{"no sign-in method", map[string]string{"GITHUB_CLIENT_ID": "", "GITHUB_CLIENT_SECRET": ""}, "at least one"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := maps.Clone(valid)
			maps.Copy(env, tt.set)
			cfg, err := loadConfig(func(k string) string { return env[k] })
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to mention %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.httpAddr != ":8080" || cfg.logLevel != slog.LevelInfo || cfg.auth.MaxDevices != 5 || len(cfg.auth.NativeRedirects) != 2 {
				t.Errorf("unexpected defaults: %q %v %d %v", cfg.httpAddr, cfg.logLevel, cfg.auth.MaxDevices, cfg.auth.NativeRedirects)
			}
		})
	}
}
