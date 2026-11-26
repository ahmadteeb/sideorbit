// Package migrations embeds the goose SQL migrations. They are append-only.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
