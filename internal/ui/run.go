package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DaanHessen/walker-tui/internal/store"
	"github.com/DaanHessen/walker-tui/internal/text"
	"github.com/DaanHessen/walker-tui/internal/util"
)

// Run boots the TUI program and blocks until it exits.
func Run(ctx context.Context, db *store.DB, narrator text.Narrator, cfg util.Config, version string) error {
	m := initialModel(ctx, db, narrator, cfg)
	program := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := program.Run()
	return err
}
