package ui

import (
	"context"
	"fmt"
	"math/rand"

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
	choices   []engine.Choice
	rng      *rand.Rand
}

func initialModel(ctx context.Context, narrator text.Narrator, seed int64) model {
	w := engine.NewWorld(seed)
	r := engine.RNG(seed)
	s := engine.NewFirstSurvivor(r, w.CurrentDay, "origin-region")
	m := model{ctx: ctx, world: w, survivor: s, narrator: narrator}
	m.styles.title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	m.rng = engine.RNG(seed)
	m.choices = engine.GenerateChoices(m.rng, m.survivor)
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
	// append choices list (simple)
	for _, c := range m.choices {
		m.md += fmt.Sprintf("\n[%d] %s (Risk: %s)", c.Index+1, c.Label, c.Risk)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
		if len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '6' {
			idx := int(msg.String()[0] - '1')
			if idx < len(m.choices) {
				c := m.choices[idx]
				engine.ApplyChoice(&m.survivor, c)
				m.choices = engine.GenerateChoices(m.rng, m.survivor)
				m.generateScene()
			}
		}
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
