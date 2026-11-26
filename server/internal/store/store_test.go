package store_test

import (
	"context"
	"testing"

	"github.com/ahmadteeb/sideorbit/server/internal/store"
	"github.com/ahmadteeb/sideorbit/server/internal/store/storetest"
)

func TestMigrate(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()

	// A restart must be a no-op.
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	tables := []string{
		"users", "identities", "refresh_tokens", "login_codes", "devices",
		"device_apps", "refresh_events", "sources", "push_subscriptions", "alerts_sent",
	}
	for _, table := range tables {
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Errorf("table %s is missing", table)
		}
	}
}
