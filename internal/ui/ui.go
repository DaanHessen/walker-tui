package ui

import (
	"context"

	"github.com/DaanHessen/walker-tui/internal/store"
	"github.com/DaanHessen/walker-tui/internal/text"
	"github.com/DaanHessen/walker-tui/internal/util"
)

// Run starts the TUI. Placeholder for now so the project builds with Postgres-only changes.
func Run(ctx context.Context, db *store.DB, narrator text.Narrator, cfg util.Config, version string) error {
	// TODO: implement Bubble Tea TUI.
	return nil
}
