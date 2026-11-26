package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := New(2, time.Minute)
	l.now = func() time.Time { return now }

	steps := []struct {
		name    string
		advance time.Duration
		key     string
		want    bool
	}{
		{"first", 0, "a", true},
		{"second", 0, "a", true},
		{"over", 0, "a", false},
		{"other key unaffected", 0, "b", true},
		{"one token back after half a window", 30 * time.Second, "a", true},
		{"and only one", 0, "a", false},
		{"full again after idle sweep", 2 * time.Minute, "a", true},
		{"burst", 0, "a", true},
		{"over again", 0, "a", false},
	}
	for _, s := range steps {
		now = now.Add(s.advance)
		if got := l.Allow(s.key); got != s.want {
			t.Errorf("%s: Allow = %v, want %v", s.name, got, s.want)
		}
	}
	if New(0, time.Minute).Allow("a") {
		t.Error("a zero limit must refuse")
	}
}
