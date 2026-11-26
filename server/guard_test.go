package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The server must never receive Apple credentials, certificates or pairing
// files (AGENTS.md hard rule 1, docs/SECURITY.md). This fails the build when
// server code starts naming them. Don't widen the allowlist without review.
var forbidden = regexp.MustCompile(`(?i)password|passwd|p12|pairing|private.?key|2fa|anisette_session`)

var allowedEverywhere = regexp.MustCompile(`(?i)pairing_?ok`) // a health flag

var allowedIn = map[string]*regexp.Regexp{
	// The operator's SMTP password.
	"config.go":             regexp.MustCompile(`SMTP_PASSWORD|\bPassword:`),
	"internal/mail/mail.go": regexp.MustCompile(`\bPassword\b|s\.Password|smtp\.PlainAuth`),
}

func offending(path, content string) (lines []string) {
	for line := range strings.Lines(content) {
		rest := allowedEverywhere.ReplaceAllString(line, "")
		if allow := allowedIn[path]; allow != nil {
			rest = allow.ReplaceAllString(rest, "")
		}
		if forbidden.MatchString(rest) {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func TestNoCredentialsInServerCode(t *testing.T) {
	scanned := 0
	root := os.DirFS(".")
	err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		if (ext != ".go" && ext != ".sql") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		scanned++
		for _, line := range offending(path, string(b)) {
			t.Errorf("%s mentions a forbidden term: %s", path, line)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 10 {
		t.Fatalf("only %d files scanned; the guard is looking in the wrong place", scanned)
	}
}

func TestGuardCatches(t *testing.T) {
	tests := []struct {
		path, line string
		want       int
	}{
		{"internal/devices/report.go", "Password string `json:\"password\"`", 1},
		{"migrations/0009.sql", "ADD COLUMN pairing_file bytea;", 1},
		{"migrations/0009.sql", "pairing_ok boolean,", 0},
		{"internal/auth/x.go", "var p12 []byte", 1},
		{"config.go", `Password: getenv("SMTP_PASSWORD"),`, 0},
		{"config.go", "applePassword := x", 1},
	}
	for _, tt := range tests {
		if got := len(offending(tt.path, tt.line)); got != tt.want {
			t.Errorf("%s %q: %d hits, want %d", tt.path, tt.line, got, tt.want)
		}
	}
}
