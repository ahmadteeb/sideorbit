package main

import (
	"log/slog"
	"maps"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	valid := map[string]string{
		"DATABASE_URL":      "postgres://u:p@db/orbit",
		"ANISETTE_UPSTREAM": "http://anisette-v3:6969",
	}
	tests := []struct {
		name      string
		set       map[string]string
		wantErr   string
		wantLevel slog.Level
	}{
		{"defaults", nil, "", slog.LevelInfo},
		{"debug", map[string]string{"LOG_LEVEL": "debug"}, "", slog.LevelDebug},
		{"no database", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL", 0},
		{"no upstream", map[string]string{"ANISETTE_UPSTREAM": ""}, "ANISETTE_UPSTREAM", 0},
		{"upstream without scheme", map[string]string{"ANISETTE_UPSTREAM": "anisette:6969"}, "ANISETTE_UPSTREAM", 0},
		{"bad level", map[string]string{"LOG_LEVEL": "loud"}, "LOG_LEVEL", 0},
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
			if cfg.httpAddr != ":8080" || cfg.logLevel != tt.wantLevel {
				t.Errorf("addr = %q level = %v", cfg.httpAddr, cfg.logLevel)
			}
		})
	}
}
