package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DaanHessen/walker-tui/internal/engine"
	"github.com/DaanHessen/walker-tui/internal/store"
	"github.com/DaanHessen/walker-tui/internal/text"
	"github.com/DaanHessen/walker-tui/internal/util"
)

// Run boots the TUI program and blocks until it exits.
func Run(ctx context.Context, db *store.DB, narrator text.Narrator, planner engine.DirectorPlanner, cfg util.Config, version string, startupErr error) error {
	m := initialModel(ctx, db, narrator, planner, cfg, startupErr)
	program := tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
