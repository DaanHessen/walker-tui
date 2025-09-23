package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/DaanHessen/walker-tui/internal/engine"
	"github.com/DaanHessen/walker-tui/internal/store"
	"github.com/DaanHessen/walker-tui/internal/text"
	"github.com/DaanHessen/walker-tui/internal/util"
)

type model struct {
	ctx      context.Context
	world    *engine.World
	survivor engine.Survivor
	narrator text.Narrator
	md       string
	styles   struct{ title lipgloss.Style }
}

func initialModel(ctx context.Context, narrator text.Narrator, seed int64) model {
	w := engine.NewWorld(seed)
	r := engine.RNG(seed)
	s := engine.NewFirstSurvivor(r, w.CurrentDay, "origin-region")
	m := model{ctx: ctx, world: w, survivor: s, narrator: narrator}
	m.styles.title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	m.generateScene()
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m *model) generateScene() {
	state := m.survivor.NarrativeState()
	scene, _ := m.narrator.Scene(m.ctx, state)
	renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
	out, _ := renderer.Render(scene)
	m.md = out
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" { return m, tea.Quit }
	}
	return m, nil
}

func (m model) View() string {
	return fmt.Sprintf("%s\n%s", m.styles.title.Render("Zero Point (Prototype)"), m.md)
}

// Run starts the TUI.
func Run(ctx context.Context, db *store.DB, narrator text.Narrator, cfg util.Config, version string) error {
	p := tea.NewProgram(initialModel(ctx, narrator, cfg.Seed))
	_, err := p.Run()
	return err
}
