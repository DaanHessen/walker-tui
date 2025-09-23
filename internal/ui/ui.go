package ui

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"gorm.io/gorm"

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
	choices  []engine.Choice
	rng      *rand.Rand
	// persistence
	db        *store.DB
	runID     uuid.UUID
	survivorID uuid.UUID
	sceneID   uuid.UUID
	turn      int
}

func initialModel(ctx context.Context, db *store.DB, narrator text.Narrator, cfg util.Config) model {
	w := engine.NewWorld(cfg.Seed)
	r := engine.RNG(cfg.Seed)
	s := engine.NewFirstSurvivor(r, w.CurrentDay, w.OriginSite)
	m := model{ctx: ctx, world: w, survivor: s, narrator: narrator, db: db}
	m.styles.title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	m.rng = engine.RNG(cfg.Seed)
	// create run + survivor records then first scene
	if err := m.bootstrapPersistence(); err != nil {
		log.Printf("bootstrap error: %v", err)
	}
	return m
}

func (m *model) bootstrapPersistence() error {
	runRepo := store.NewRunRepo(m.db)
	survRepo := store.NewSurvivorRepo(m.db)
	run, err := runRepo.Create(m.ctx, m.world.OriginSite, m.world.Seed)
	if err != nil { return err }
	m.runID = run.ID
	sid, err := survRepo.Create(m.ctx, m.runID, m.survivor)
	if err != nil { return err }
	m.survivorID = sid
	return m.newSceneTx()
}

func (m *model) newSceneTx() error {
	return m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		// generate choices + scene markdown
		m.choices = engine.GenerateChoices(m.rng, m.survivor)
		state := m.survivor.NarrativeState()
		md, _ := m.narrator.Scene(m.ctx, state)
		sceneRepo := store.NewSceneRepo(m.db)
		choiceRepo := store.NewChoiceRepo(m.db)
		sceneID, err := sceneRepo.Insert(m.ctx, tx, m.runID, m.survivorID, m.world.CurrentDay, "day", m.survivor.Environment.LAD, md)
		if err != nil { return err }
		if err := choiceRepo.BulkInsert(m.ctx, tx, sceneID, m.choices); err != nil { return err }
		m.sceneID = sceneID
		// render for view including choices list
		renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
		out, _ := renderer.Render(md)
		for _, c := range m.choices {
			out += fmt.Sprintf("\n[%d] %s (Risk: %s)", c.Index+1, c.Label, c.Risk)
		}
		m.md = out
		return nil
	})
}

// generateScene no longer used externally; kept for reference (unused)
// func (m *model) generateScene() { }

func (m model) Init() tea.Cmd { return nil }

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
				// apply choice & persist outcome transactionally
				if err := m.resolveChoiceTx(c); err != nil {
					log.Printf("turn error: %v", err)
				}
			}
		}
	}
	return m, nil
}

func (m *model) resolveChoiceTx(c engine.Choice) error {
	var aliveAfter bool
	var outcomeMD string
	prevTurn := m.turn
	// perform transactional persistence of update & outcome
	err := m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		delta := engine.ApplyChoice(&m.survivor, c)
		updateRepo := store.NewUpdateRepo(m.db)
		outcomeRepo := store.NewOutcomeRepo(m.db)
		survRepo := store.NewSurvivorRepo(m.db)
		logRepo := store.NewLogRepo(m.db)
		if _, err := updateRepo.Insert(m.ctx, tx, m.sceneID, delta, m.survivor.Conditions); err != nil { return err }
		outMD, _ := m.narrator.Outcome(m.ctx, m.survivor.NarrativeState(), c, delta)
		if _, err := outcomeRepo.Insert(m.ctx, tx, m.sceneID, outMD); err != nil { return err }
		if err := survRepo.Update(m.ctx, tx, m.survivorID, m.survivor); err != nil { return err }
		outcomeMD = outMD
		aliveAfter = m.survivor.Alive
		// simplistic choices summary per turn appended as new master log row
		_ , _ = logRepo.Insert(m.ctx, tx, m.runID, m.survivorID, map[string]any{"turn": prevTurn, "choice": c.Label, "delta": delta}, "(recap placeholder)")
		if !aliveAfter { // archive card
			archRepo := store.NewArchiveRepo(m.db)
			if _, err := archRepo.Insert(m.ctx, tx, m.runID, m.survivorID, m.world.CurrentDay, m.survivor.Region, "unknown", m.survivor.Inventory, "# Archive Card\n(placeholder)"); err != nil { return err }
		}
		return nil
	})
	if err != nil { return err }
	m.turn++
	// Post-commit: if dead show final outcome, else proceed to next scene
	if !aliveAfter {
		m.md = m.md + "\n\n" + outcomeMD + "\n\nSurvivor has perished. Run ends (prototype). Press q to quit."
		return nil
	}
	// alive: generate next scene (persist) and prepend outcome from previous turn for continuity
	prev := m.md + "\n\n" + outcomeMD + "\n\n--- NEXT ---\n"
	if err := m.newSceneTx(); err != nil { return err }
	m.md = prev + m.md
	return nil
}

func (m model) View() string {
	return fmt.Sprintf("%s\n%s", m.styles.title.Render("Zero Point (Prototype)"), m.md)
}

// Run starts the TUI.
func Run(ctx context.Context, db *store.DB, narrator text.Narrator, cfg util.Config, version string) error {
	p := tea.NewProgram(initialModel(ctx, db, narrator, cfg))
	_, err := p.Run()
	return err
}
