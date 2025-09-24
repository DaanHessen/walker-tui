package ui

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

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
	// sceneRendered holds the rendered markdown for the current scene ONLY (without appended choice lines)
	sceneRendered string
	styles        struct{ title lipgloss.Style }
	choices       []engine.Choice
	rng           *rand.Rand
	// persistence
	db           *store.DB
	runID        uuid.UUID
	survivorID   uuid.UUID
	sceneID      uuid.UUID
	turn         int
	deaths       int    // number of deceased survivors in this run
	view         string // "scene" | "log" | "archive" | "help"
	logs         []store.MasterLog
	archives     []store.ArchiveCard
	settings     store.Settings
	exportStatus string
}

func initialModel(ctx context.Context, db *store.DB, narrator text.Narrator, cfg util.Config) model {
	w := engine.NewWorld(cfg.Seed)
	r := engine.RNG(cfg.Seed)
	s := engine.NewFirstSurvivor(r, w.CurrentDay, w.OriginSite)
	m := model{ctx: ctx, world: w, survivor: s, narrator: narrator, db: db}
	m.styles.title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	m.rng = engine.RNG(cfg.Seed)
	m.view = "scene"
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
	if err != nil {
		return err
	}
	m.runID = run.ID
	sid, err := survRepo.Create(m.ctx, m.runID, m.survivor)
	if err != nil {
		return err
	}
	m.survivorID = sid
	if err := m.newSceneTx(); err != nil {
		return err
	}
	// ensure default settings row
	setRepo := store.NewSettingsRepo(m.db)
	_ = setRepo.Upsert(m.ctx, m.runID, false, "standard", "en", "auto")
	m.refreshSettings()
	return nil
}

func (m *model) newSceneTx() error {
	return m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		m.choices = engine.GenerateChoices(m.rng, m.survivor, engine.WithScarcity(m.settings.Scarcity), engine.WithTextDensity(m.settings.TextDensity), engine.WithInfectedPresent(m.survivor.Environment.Infected))
		state := m.survivor.NarrativeState()
		md, _ := m.narrator.Scene(m.ctx, state)
		sceneRepo := store.NewSceneRepo(m.db)
		choiceRepo := store.NewChoiceRepo(m.db)
		sceneID, err := sceneRepo.Insert(m.ctx, tx, m.runID, m.survivorID, m.world.CurrentDay, "day", m.survivor.Environment.LAD, md)
		if err != nil {
			return err
		}
		if err := choiceRepo.BulkInsert(m.ctx, tx, sceneID, m.choices); err != nil {
			return err
		}
		m.sceneID = sceneID
		// render for view including choices list
		renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
		rendered, _ := renderer.Render(md)
		m.sceneRendered = rendered
		m.md = m.buildSceneWithChoices()
		return nil
	})
}

// generateScene no longer used externally; kept for reference (unused)
// func (m *model) generateScene() { }

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := msg.String()
		if k == "ctrl+c" || k == "q" {
			return m, tea.Quit
		}
		switch k {
		case "tab":
			m.cyclePrimaryViews()
		case "l":
			m.view = "log"
			m.refreshLogs()
		case "a":
			m.view = "archive"
			m.refreshArchives()
		case "s":
			m.view = "settings"
			m.refreshSettings()
		case "?":
			if m.view == "help" {
				m.view = "scene"
			} else {
				m.view = "help"
			}
		case "t":
			if m.view == "settings" {
				m.toggleScarcity()
			}
		case "d":
			if m.view == "settings" {
				m.cycleDensity()
			}
		case "e":
			m.exportRun()
		default:
			if m.view == "scene" && len(k) == 1 && k[0] >= '1' && k[0] <= '6' {
				idx := int(k[0] - '1')
				if idx < len(m.choices) {
					_ = m.resolveChoiceTx(m.choices[idx])
				}
			}
		}
	}
	return m, nil
}

func (m *model) cyclePrimaryViews() {
	order := []string{"scene", "log", "archive", "settings"}
	cur := 0
	for i, v := range order {
		if v == m.view {
			cur = i
			break
		}
	}
	next := order[(cur+1)%len(order)]
	m.view = next
	if next == "log" {
		m.refreshLogs()
	} else if next == "archive" {
		m.refreshArchives()
	} else if next == "settings" {
		m.refreshSettings()
	}
}

func (m *model) refreshLogs() {
	lr := store.NewLogRepo(m.db)
	logs, err := lr.ListRecent(m.ctx, m.runID, 20)
	if err != nil {
		return
	}
	m.logs = logs
}

func (m *model) refreshArchives() {
	ar := store.NewArchiveRepo(m.db)
	acs, err := ar.List(m.ctx, m.runID, 20)
	if err != nil {
		return
	}
	m.archives = acs
}

func (m *model) refreshSettings() {
	setRepo := store.NewSettingsRepo(m.db)
	if s, err := setRepo.Get(m.ctx, m.runID); err == nil {
		m.settings = s
	}
}

func (m *model) toggleScarcity() {
	setRepo := store.NewSettingsRepo(m.db)
	if err := setRepo.ToggleScarcity(m.ctx, m.runID); err == nil {
		m.refreshSettings()
		m.forceRegenerateChoices()
	}
}

func (m *model) cycleDensity() {
	setRepo := store.NewSettingsRepo(m.db)
	if err := setRepo.CycleDensity(m.ctx, m.runID); err == nil {
		m.refreshSettings()
		m.forceRegenerateChoices()
	}
}

func (m *model) forceRegenerateChoices() {
	m.choices = engine.GenerateChoices(m.rng, m.survivor, engine.WithScarcity(m.settings.Scarcity), engine.WithTextDensity(m.settings.TextDensity), engine.WithInfectedPresent(m.survivor.Environment.Infected))
	m.md = m.buildSceneWithChoices()
}

// buildSceneWithChoices composes the current scene base markdown plus the active choices list.
func (m *model) buildSceneWithChoices() string {
	out := m.sceneRendered
	for _, c := range m.choices {
		out += fmt.Sprintf("\n[%d] %s (Risk: %s)", c.Index+1, c.Label, c.Risk)
	}
	return out
}

func (m *model) resolveChoiceTx(c engine.Choice) error {
	var aliveAfter bool
	var outcomeMD string
	prevTurn := m.turn
	var deathCause string
	// perform transactional persistence of update & outcome
	err := m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		delta := engine.ApplyChoice(&m.survivor, c)
		updateRepo := store.NewUpdateRepo(m.db)
		outcomeRepo := store.NewOutcomeRepo(m.db)
		survRepo := store.NewSurvivorRepo(m.db)
		logRepo := store.NewLogRepo(m.db)
		if _, err := updateRepo.Insert(m.ctx, tx, m.sceneID, delta, m.survivor.Conditions); err != nil {
			return err
		}
		outMD, _ := m.narrator.Outcome(m.ctx, m.survivor.NarrativeState(), c, delta)
		if _, err := outcomeRepo.Insert(m.ctx, tx, m.sceneID, outMD); err != nil {
			return err
		}
		if err := survRepo.Update(m.ctx, tx, m.survivorID, m.survivor); err != nil {
			return err
		}
		outcomeMD = outMD
		aliveAfter = m.survivor.Alive
		recap := buildRecap(outMD)
		_, _ = logRepo.Insert(m.ctx, tx, m.runID, m.survivorID, map[string]any{"turn": prevTurn, "choice": c.Label, "delta": delta}, recap)
		if !aliveAfter { // archive card
			deathCause = classifyDeath(m.survivor)
			archRepo := store.NewArchiveRepo(m.db)
			// derive simple key skills: top 3 skills with >0 level (placeholder currently all zero so none)
			var keySkills []string
			for sk, lvl := range m.survivor.Skills {
				if lvl > 0 {
					keySkills = append(keySkills, string(sk))
				}
				if len(keySkills) == 3 {
					break
				}
			}
			// notable decisions: last 5 log recaps (placeholder using labels previously chosen)
			lr := store.NewLogRepo(m.db)
			recent, _ := lr.ListRecent(m.ctx, m.runID, 5)
			var notable []string
			for _, r := range recent {
				notable = append(notable, r.NarrativeRecap)
			}
			// allies placeholder: none for now
			allies := []string{}
			cardMD := fmt.Sprintf("# Archive Card\nName: %s\nCause: %s\nDay: %d\nRegion: %s\nInventory FoodDays: %.2f Water: %.2f\n", m.survivor.Name, deathCause, m.world.CurrentDay, m.survivor.Region, m.survivor.Inventory.FoodDays, m.survivor.Inventory.WaterLiters)
			if _, err := archRepo.Insert(m.ctx, tx, m.runID, m.survivorID, m.world.CurrentDay, m.survivor.Region, deathCause, keySkills, notable, allies, m.survivor.Inventory, cardMD); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	m.turn++
	prev := m.md + "\n\n" + outcomeMD
	if !aliveAfter {
		m.deaths++
		if err := m.spawnReplacementTx(); err != nil {
			m.md = prev + "\n\nSurvivor has perished (" + deathCause + "). Replacement failed: " + err.Error() + "\nPress q to quit."
			return nil
		}
		m.md = prev + fmt.Sprintf("\n\n--- Survivor died (%s, total deaths: %d). New survivor spawned. ---\n\n", deathCause, m.deaths) + m.md
		return nil
	}
	prev += "\n\n--- NEXT ---\n"
	if err := m.newSceneTx(); err != nil {
		return err
	}
	m.md = prev + m.md
	return nil
}

// spawnReplacementTx creates a new generic survivor and first scene transactionally.
func (m *model) spawnReplacementTx() error {
	return m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		r := engine.RNG(m.world.Seed + int64(m.deaths) + 1)
		newSurv := engine.NewGenericSurvivor(r, m.world.CurrentDay, m.world.OriginSite)
		// persist survivor
		survRepo := store.NewSurvivorRepo(m.db)
		sid, err := survRepo.Create(m.ctx, m.runID, newSurv)
		if err != nil {
			return err
		}
		m.survivor = newSurv
		m.survivorID = sid
		// create opening scene for new survivor
		m.choices = engine.GenerateChoices(m.rng, m.survivor, engine.WithScarcity(m.settings.Scarcity), engine.WithTextDensity(m.settings.TextDensity), engine.WithInfectedPresent(m.survivor.Environment.Infected))
		state := m.survivor.NarrativeState()
		md, _ := m.narrator.Scene(m.ctx, state)
		sceneRepo := store.NewSceneRepo(m.db)
		choiceRepo := store.NewChoiceRepo(m.db)
		sceneID, err := sceneRepo.Insert(m.ctx, tx, m.runID, m.survivorID, m.world.CurrentDay, "day", m.survivor.Environment.LAD, md)
		if err != nil {
			return err
		}
		if err := choiceRepo.BulkInsert(m.ctx, tx, sceneID, m.choices); err != nil {
			return err
		}
		m.sceneID = sceneID
		renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
		rendered, _ := renderer.Render(md)
		m.sceneRendered = rendered
		m.md = m.buildSceneWithChoices()
		return nil
	})
}

func (m model) View() string {
	head := ""
	if m.exportStatus != "" {
		head = fmt.Sprintf("[Export] %s\n", m.exportStatus)
	}
	switch m.view {
	case "log":
		var b strings.Builder
		b.WriteString(m.styles.title.Render("Zero Point (Logs)") + "\n(Tab cycle | A archive | ? help | Q quit)\n\n")
		for _, l := range m.logs {
			b.WriteString(fmt.Sprintf("- %s\n", l.NarrativeRecap))
		}
		if len(m.logs) == 0 {
			b.WriteString("(no log entries yet)\n")
		}
		return b.String()
	case "archive":
		var b strings.Builder
		b.WriteString(m.styles.title.Render("Zero Point (Archives)") + "\n(Tab cycle | L logs | ? help | Q quit)\n\n")
		if len(m.archives) == 0 {
			b.WriteString("(no archive cards yet)\n")
		} else {
			for _, a := range m.archives {
				b.WriteString(fmt.Sprintf("Day %d – %s – %s\n", a.WorldDay, a.Region, a.CauseOfDeath))
				b.WriteString("  " + strings.ReplaceAll(a.Markdown, "\n", " ")[:min(80, len(a.Markdown))] + "...\n")
			}
		}
		return b.String()
	case "settings":
		return head + m.styles.title.Render("Zero Point (Settings)") + fmt.Sprintf("\nScarcity: %v (t toggle)\nDensity: %s (d cycle)\nLanguage: %s\nNarrator: %s\n\n(E export) (Tab cycle | L logs | A archives | S settings | ? help | Q quit)", m.settings.Scarcity, m.settings.TextDensity, m.settings.Language, m.settings.Narrator)
	case "help":
		return head + m.styles.title.Render("Zero Point (Help)") + "\nKeys: 1-6 choose | Tab cycle views | L logs | A archives | S settings | E export | T toggle scarcity | D cycle density | ? help | Q quit.\nPress ? to close."
	default:
		return head + fmt.Sprintf("%s\n%s\n\n(E export | Tab cycle | L logs | A archives | S settings | ? help)", m.styles.title.Render("Zero Point (Prototype)"), m.md)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildRecap extracts a short single-line summary from outcome markdown.
func buildRecap(md string) string {
	clean := strings.ReplaceAll(md, "\n", " ")
	clean = strings.TrimSpace(clean)
	if len(clean) > 140 {
		clean = clean[:140] + "..."
	}
	return clean
}

// classifyDeath infers a coarse cause based on terminal stats.
func classifyDeath(s engine.Survivor) string {
	if s.Stats.Health <= 0 {
		if s.Stats.Thirst >= 95 {
			return "dehydration"
		}
		if s.Stats.Hunger >= 95 {
			return "starvation"
		}
		if s.Stats.Fatigue >= 95 {
			return "collapse"
		}
		return "injury"
	}
	return "unknown"
}

// Run starts the TUI.
func Run(ctx context.Context, db *store.DB, narrator text.Narrator, cfg util.Config, version string) error {
	p := tea.NewProgram(initialModel(ctx, db, narrator, cfg))
	_, err := p.Run()
	return err
}

func (m *model) exportRun() {
	repo := store.NewSceneRepo(m.db)
	scenes, err := repo.ScenesWithOutcomes(m.ctx, m.runID)
	if err != nil {
		m.exportStatus = "failed: collect"
		return
	}
	var b strings.Builder
	b.WriteString("# Zero Point Run Export\n")
	for _, sc := range scenes {
		b.WriteString(fmt.Sprintf("\n## Day %d Scene\n%s\n", sc.WorldDay, sc.SceneMD))
		if sc.OutcomeMD != "" {
			b.WriteString("\n### Outcome\n" + sc.OutcomeMD + "\n")
		}
	}
	dir := filepath.Join(os.Getenv("HOME"), ".zero-point", "exports")
	_ = os.MkdirAll(dir, 0o755)
	fname := fmt.Sprintf("run_%s.md", m.runID.String())
	path := filepath.Join(dir, fname)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		m.exportStatus = "failed: write"
		return
	}
	m.exportStatus = path
}
