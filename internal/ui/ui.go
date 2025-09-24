package ui

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

const (
	viewMainMenu    = "main_menu"
	viewScene       = "scene"
	viewLog         = "log"
	viewArchive     = "archive"
	viewSettings    = "settings"
	viewHelp        = "help"
	viewWorldConfig = "world_config"
	viewTimeline    = "timeline"
)

type model struct {
	ctx      context.Context
	world    *engine.World
	survivor engine.Survivor
	narrator text.Narrator
	md       string
	// sceneRendered: current scene markdown (no appended choices)
	sceneRendered string
	// accumulated timeline (previous scenes+outcomes)
	timeline string
	styles   struct{ title lipgloss.Style }
	choices  []engine.Choice
	rng      *rand.Rand
	// persistence
	db           *store.DB
	runID        uuid.UUID
	survivorID   uuid.UUID
	sceneID      uuid.UUID
	turn         int
	deaths       int
	view         string
	logs         []store.MasterLog
	archives     []store.ArchiveCard
	settings     store.Settings
	exportStatus string
	debugLAD     bool
	// custom action
	customInput   string
	customEnabled bool
	customStatus  string
	width         int
	height        int

	// pre-run configuration
	preRunScarcity bool
	preRunDensity  string
	preRunSeed     int64

	// archive browser
	archiveIndex  int
	archiveDetail bool

	// scrolling support
	scrollOffset int
	maxScroll    int

	// multi-day loop
	scenesToday int

	// timeline scrolling
	timelineScroll int
}

// initialModel boots to main menu; game state seeded but not persisted until New Game selected.
func initialModel(ctx context.Context, db *store.DB, narrator text.Narrator, cfg util.Config) model {
	w := engine.NewWorld(cfg.Seed)
	r := engine.RNG(cfg.Seed)
	s := engine.NewFirstSurvivor(r, w.CurrentDay, w.OriginSite)
	w.CurrentDay = s.Environment.WorldDay
	m := model{ctx: ctx, world: w, survivor: s, narrator: narrator, db: db, debugLAD: cfg.DebugLAD}
	m.styles.title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	m.rng = engine.RNG(cfg.Seed)
	m.view = viewMainMenu
	m.preRunScarcity = false
	m.preRunDensity = "standard"
	m.preRunSeed = cfg.Seed
	return m
}

// bootstrapPersistence creates run, survivor, initial scene & choices.
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
	setRepo := store.NewSettingsRepo(m.db)
	_ = setRepo.UpsertLegacy(m.ctx, m.runID, m.preRunScarcity, m.preRunDensity, "en", "auto")
	m.refreshSettings()
	return nil
}

// startNewGame persists and enters scene view.
func (m *model) startNewGame() {
	// apply pre-run seed settings (reseed world + rng if user changed)
	m.world.Seed = m.preRunSeed
	m.rng = engine.RNG(m.preRunSeed)
	// recreate first survivor with potentially new seed
	r := engine.RNG(m.preRunSeed)
	m.survivor = engine.NewFirstSurvivor(r, m.world.CurrentDay, m.world.OriginSite)
	m.world.CurrentDay = m.survivor.Environment.WorldDay
	if err := m.bootstrapPersistence(); err != nil {
		m.md = "Failed to start new game: " + err.Error()
	} else {
		m.view = viewScene
	}
}

// continueGame loads latest run & alive survivor, then generates a fresh scene (simplified resume).
func (m *model) continueGame() {
	rr := store.NewRunRepo(m.db)
	run, err := rr.GetLatestRun(m.ctx)
	if err != nil {
		m.md = "No run to continue"
		return
	}
	sr := store.NewSurvivorRepo(m.db)
	surv, sid, err := sr.GetAliveSurvivor(m.ctx, run.ID)
	if err != nil {
		m.md = "No alive survivor"
		return
	}
	m.world.Seed = run.Seed
	m.world.OriginSite = run.OriginSite
	m.world.CurrentDay = run.CurrentDay
	m.survivor = surv
	m.survivorID = sid
	m.runID = run.ID
	m.refreshSettings()
	_ = m.newSceneTx()
	m.view = viewScene
}

// newSceneTx inserts scene + choices transactionally, renders markdown.
func (m *model) newSceneTx() error {
	return m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		m.choices = engine.GenerateChoices(m.rng, m.survivor, engine.WithScarcity(m.settings.Scarcity), engine.WithTextDensity(m.settings.TextDensity), engine.WithInfectedPresent(m.survivor.Environment.Infected), engine.WithDifficulty(engine.Difficulty(m.settings.Difficulty)))
		m.customEnabled = true
		m.customStatus = ""
		// cooldown check will happen in handleCustomAction
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
		m.md = m.buildGameView()
		return nil
	})
}

// tea.Model implementation ---------------------------------------------------
func (m model) Init() tea.Cmd { return nil }

func (m model) View() string {
	if m.view == viewMainMenu {
		return m.renderMainMenu()
	}
	if m.view == viewWorldConfig {
		return m.renderWorldConfig()
	}
	if m.view == viewScene {
		return m.renderSceneLayout()
	}
	if m.view == viewTimeline {
		return m.renderTimeline()
	}
	switch m.view {
	case viewLog:
		var b strings.Builder
		b.WriteString(m.styles.title.Render("Zero Point (Logs)") + "\n")
		for _, l := range m.logs {
			b.WriteString("- " + l.NarrativeRecap + "\n")
		}
		if len(m.logs) == 0 {
			b.WriteString("(no log entries)\n")
		}
		return b.String()
	case viewArchive:
		return m.renderArchive()
	case viewSettings:
		return m.renderSettings()
	case viewHelp:
		return m.renderHelp()
	default:
		return m.md
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.view == viewScene {
			m.md = m.renderSceneLayout()
		}
		return m, nil
	case tea.KeyMsg:
		k := msg.String()
		// timeline navigation
		if m.view == viewTimeline {
			switch k {
			case "pgdown", "ctrl+f":
				m.timelineScroll += 12
			case "pgup", "ctrl+b":
				m.timelineScroll -= 12
			case "down", "j":
				m.timelineScroll += 3
			case "up", "k":
				m.timelineScroll -= 3
			case "home":
				m.timelineScroll = 0
			case "end":
				m.timelineScroll = 1 << 30
			case "esc", "q":
				m.view = viewScene
			}
			if m.timelineScroll < 0 {
				m.timelineScroll = 0
			}
			return m, nil
		}
		// global key for timeline
		if k == "y" {
			m.view = viewTimeline
			return m, nil
		}
		if m.view == viewMainMenu {
			switch k {
			case "1":
				m.startNewGame()
			case "2":
				m.continueGame()
			case "3":
				m.view = viewWorldConfig
			case "5":
				m.view = viewHelp
			}
			return m, nil
		}
		if m.view == viewWorldConfig {
			switch k {
			case "1":
				m.preRunScarcity = !m.preRunScarcity
			case "2":
				m.preRunDensity = cycleDensityLocal(m.preRunDensity)
			case "3":
				m.preRunSeed = time.Now().UnixNano()
			case "4", "esc":
				m.view = viewMainMenu
			}
			return m, nil
		}
		if m.view == viewArchive {
			switch k {
			case "up", "k":
				if m.archiveIndex > 0 {
					m.archiveIndex--
				}
			case "down", "j":
				if m.archiveIndex < len(m.archives)-1 {
					m.archiveIndex++
				}
			case "enter":
				m.archiveDetail = !m.archiveDetail
			case "esc", "q":
				if m.archiveDetail {
					m.archiveDetail = false
				} else {
					m.view = viewScene
				}
			}
			return m, nil
		}
		if m.view == viewSettings {
			switch k {
			case "n":
				m.toggleNarrator()
			case "g":
				m.cycleLanguage()
			}
			return m, nil
		}
		switch k {
		case "tab":
			m.cyclePrimaryViews()
		case "l":
			m.view = viewLog
			m.refreshLogs()
		case "a":
			m.view = viewArchive
			m.refreshArchives()
		case "s":
			m.view = viewSettings
			m.refreshSettings()
		case "?":
			if m.view == viewHelp {
				m.view = viewScene
			} else {
				m.view = viewHelp
			}
		case "t":
			if m.view == viewSettings {
				m.toggleScarcity()
			}
		case "d":
			if m.view == viewSettings {
				m.cycleDensity()
			}
		case "e":
			m.exportRun()
		case "f6":
			m.debugLAD = !m.debugLAD
		default:
			if m.view == viewScene {
				if k == "enter" && m.customEnabled {
					m.handleCustomAction()
					return m, nil
				}
				if len(k) == 1 && k[0] >= '1' && k[0] <= '6' {
					idx := int(k[0] - '1')
					if idx < len(m.choices) {
						_ = m.resolveChoiceTx(m.choices[idx])
					}
					return m, nil
				}
				if len(k) == 1 && m.customEnabled && isRuneInput(k) {
					m.customInput += k
					m.md = m.buildGameView()
					return m, nil
				}
				if k == "backspace" && len(m.customInput) > 0 {
					m.customInput = m.customInput[:len(m.customInput)-1]
					m.md = m.buildGameView()
					return m, nil
				}
			}
		}
		if m.view == viewScene {
			switch k {
			case "pgdown", "ctrl+f":
				m.scrollOffset += 8
			case "pgup", "ctrl+b":
				m.scrollOffset -= 8
			case "home":
				m.scrollOffset = 0
			case "end":
				m.scrollOffset = m.maxScroll
			}
		}
	}
	return m, nil
}

// Layout rendering -----------------------------------------------------------
func (m *model) renderSceneLayout() string {
	// Dimensions
	w := m.width
	if w <= 0 {
		w = 100
	}
	sidebarWidth := 30
	if w < 90 {
		sidebarWidth = 24
	}
	mainWidth := w - sidebarWidth - 1

	// Build components
	top := m.renderTopBar()
	mainRaw := m.buildMainScene()
	lines := strings.Split(mainRaw, "\n")
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if m.scrollOffset > len(lines) {
		m.scrollOffset = len(lines)
	}
	viewLines := lines
	availHeight := m.height - 4 // approximate (top+bottom)
	if availHeight > 5 && len(lines) > availHeight {
		if m.scrollOffset+availHeight > len(lines) {
			m.scrollOffset = len(lines) - availHeight
		}
		viewLines = lines[m.scrollOffset : m.scrollOffset+availHeight]
		m.maxScroll = len(lines) - availHeight
	}
	main := lipgloss.NewStyle().Width(mainWidth).Render(strings.Join(viewLines, "\n"))
	side := lipgloss.NewStyle().Width(sidebarWidth).Border(lipgloss.NormalBorder()).Padding(0, 1).Render(m.buildSidebar())
	body := lipgloss.JoinHorizontal(lipgloss.Top, main, side)
	bottom := m.renderBottomBar()
	return lipgloss.JoinVertical(lipgloss.Left, top, body, bottom)
}

func (m *model) renderTopBar() string {
	name := m.survivor.Name
	day := fmt.Sprintf("Day %d", m.survivor.Environment.WorldDay)
	lad := fmt.Sprintf("LAD %d", m.survivor.Environment.LAD)
	if m.debugLAD {
		lad += fmt.Sprintf(" inf:%v", m.survivor.Environment.Infected)
	}
	loc := fmt.Sprintf("%s/%s", m.survivor.Region, m.survivor.Location)
	parts := []string{"Zero Point", name, day, lad, loc, m.survivor.Environment.TimeOfDay}
	bar := strings.Join(parts, " | ")
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render(bar)
}

func (m *model) renderBottomBar() string {
	left := "[1-6] choose  [Enter] commit custom  [Tab] cycle  [L] logs  [A] archive  [S] settings  [?] help  [Q] quit"
	cust := m.customInput
	if !m.customEnabled {
		cust = "(disabled)"
	}
	custom := "Custom> " + cust
	if m.customStatus != "" {
		custom += " [" + m.customStatus + "]"
	}
	export := ""
	if m.exportStatus != "" {
		export = " export:" + m.exportStatus
	}
	line := custom + "  " + export
	w := m.width
	if w <= 0 {
		w = 100
	}
	if len(line) > w {
		if w > 10 {
			line = line[:w-3] + "..."
		}
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(left + "\n" + line)
}

// Build main scene area (scene + choices)
func (m *model) buildMainScene() string {
	var b strings.Builder
	b.WriteString("# SCENE\n")
	b.WriteString(m.sceneRendered)
	b.WriteString("\n\n# CHOICES\n")
	for _, c := range m.choices {
		b.WriteString(fmt.Sprintf("[%d] %s | Cost:%s | Risk:%s\n", c.Index+1, c.Label, formatCost(c.Cost), c.Risk))
	}
	return b.String()
}

// Sidebar condensed sections
func (m *model) buildSidebar() string {
	s := m.survivor
	var b strings.Builder
	b.WriteString("CHARACTER\n")
	b.WriteString(fmt.Sprintf("%s (%s)\n", s.Name, s.Background))
	b.WriteString(fmt.Sprintf("Age %d  %s\n", s.Age, s.Environment.TimeOfDay))
	b.WriteString(fmt.Sprintf("%s\n\n", s.Location))
	// Stats bars condensed
	b.WriteString("STATS\n")
	b.WriteString(sb("H", s.Stats.Health))
	b.WriteString(sb("Hu", s.Stats.Hunger))
	b.WriteString(sb("Th", s.Stats.Thirst))
	b.WriteString(sb("Fa", s.Stats.Fatigue))
	b.WriteString(sb("Mo", s.Stats.Morale) + "\n\n")
	// Inventory
	inv := s.Inventory
	b.WriteString("INV\n")
	b.WriteString(fmt.Sprintf("Food %.1fd\nWater %.1fL\nMed %d  Wpn %d\n\n", inv.FoodDays, inv.WaterLiters, len(inv.Medical), len(inv.Weapons)))
	// Conditions
	b.WriteString("COND\n")
	if len(s.Conditions) == 0 {
		b.WriteString("none\n\n")
	} else {
		for _, c := range s.Conditions {
			b.WriteString(string(c) + " ")
		}
		b.WriteString("\n\n")
	}
	// Skills grid all
	b.WriteString("SKILLS\n")
	if len(s.Skills) == 0 {
		b.WriteString("(none)\n")
	} else {
		names := make([]string, 0, len(s.Skills))
		vals := map[string]int{}
		for k, v := range s.Skills {
			names = append(names, string(k))
			vals[string(k)] = v
		}
		sort.Strings(names)
		for _, name := range names {
			b.WriteString(fmt.Sprintf("%s:%d ", abbrev(name), vals[name]))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func sb(label string, v int) string { return fmt.Sprintf("%-2s %s %3d\n", label, bar(v), v) }

// Main menu rendering.
func (m *model) renderMainMenu() string {
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Width(50)
	content := "ZERO POINT — MAIN MENU\n\n[1] New Game\n[2] Continue Game\n[3] World Settings\n[4] Survivor Archive (TBD)\n[5] About / Rules\n\nQ Quit"
	return box.Render(content)
}

func (m *model) renderWorldConfig() string {
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Width(60)
	scar := "Off"
	if m.preRunScarcity {
		scar = "On"
	}
	content := fmt.Sprintf("WORLD SETTINGS (Pre-Run)\n\nSeed: %d\nScarcity: %s (1 toggle)\nText Density: %s (2 cycle)\n[3] Regenerate Seed\n[4] Back\n", m.preRunSeed, scar, m.preRunDensity)
	return box.Render(content)
}

func cycleDensityLocal(cur string) string {
	switch cur {
	case "concise":
		return "standard"
	case "standard":
		return "rich"
	default:
		return "concise"
	}
}

// Re-added utility helpers and game loop pieces -----------------------------
func (m *model) cyclePrimaryViews() {
	order := []string{viewScene, viewLog, viewArchive, viewSettings}
	cur := 0
	for i, v := range order {
		if v == m.view {
			cur = i
			break
		}
	}
	m.view = order[(cur+1)%len(order)]
	switch m.view {
	case viewLog:
		m.refreshLogs()
	case viewArchive:
		m.refreshArchives()
	case viewSettings:
		m.refreshSettings()
	}
}

func (m *model) refreshLogs() {
	lr := store.NewLogRepo(m.db)
	if logs, err := lr.ListRecent(m.ctx, m.runID, 40); err == nil {
		m.logs = logs
	}
}
func (m *model) refreshArchives() {
	ar := store.NewArchiveRepo(m.db)
	if acs, err := ar.List(m.ctx, m.runID, 40); err == nil {
		m.archives = acs
		if m.archiveIndex >= len(m.archives) {
			m.archiveIndex = len(m.archives) - 1
		}
		if m.archiveIndex < 0 {
			m.archiveIndex = 0
		}
	}
}
func (m *model) refreshSettings() {
	sr := store.NewSettingsRepo(m.db)
	if s, err := sr.Get(m.ctx, m.runID); err == nil {
		m.settings = s
	}
}
func (m *model) toggleScarcity() {
	sr := store.NewSettingsRepo(m.db)
	if err := sr.ToggleScarcity(m.ctx, m.runID); err == nil {
		m.refreshSettings()
		m.forceRegenerateChoices()
	}
}
func (m *model) cycleDensity() {
	sr := store.NewSettingsRepo(m.db)
	if err := sr.CycleDensity(m.ctx, m.runID); err == nil {
		m.refreshSettings()
		m.forceRegenerateChoices()
	}
}

func (m *model) forceRegenerateChoices() {
	m.choices = engine.GenerateChoices(m.rng, m.survivor, engine.WithScarcity(m.settings.Scarcity), engine.WithTextDensity(m.settings.TextDensity), engine.WithInfectedPresent(m.survivor.Environment.Infected), engine.WithDifficulty(engine.Difficulty(m.settings.Difficulty)))
	m.md = m.renderSceneLayout()
}

func (m *model) handleCustomAction() {
	// enforce 2-scene cooldown since last custom (stored turn number)
	if m.survivor.Meters != nil {
		if last, ok := m.survivor.Meters[engine.MeterCustomLastTurn]; ok {
			if m.turn-last < 2 { // need at least 2 full scenes gap
				m.customStatus = "cooldown"
				m.customEnabled = false
				m.md = m.renderSceneLayout()
				return
			}
		}
	}
	choice, ok, reason := engine.ValidateCustomAction(m.customInput, m.survivor)
	if !ok {
		m.customStatus = reason
		m.customEnabled = false
		m.md = m.renderSceneLayout()
		return
	}
	_ = m.resolveChoiceTx(choice)
	m.customInput = ""
	m.customStatus = ""
}

// resolveChoiceTx (replacing earlier version with new layout integration)
func (m *model) resolveChoiceTx(c engine.Choice) error {
	var aliveAfter bool
	var outcomeMD string
	prevTurn := m.turn
	var deathCause string
	err := m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		delta := engine.ApplyChoice(&m.survivor, c, engine.Difficulty(m.settings.Difficulty), m.turn)
		upRepo := store.NewUpdateRepo(m.db)
		outRepo := store.NewOutcomeRepo(m.db)
		survRepo := store.NewSurvivorRepo(m.db)
		logRepo := store.NewLogRepo(m.db)
		if _, err := upRepo.Insert(m.ctx, tx, m.sceneID, delta, m.survivor.Conditions); err != nil {
			return err
		}
		outMD, _ := m.narrator.Outcome(m.ctx, m.survivor.NarrativeState(), c, delta)
		if _, err := outRepo.Insert(m.ctx, tx, m.sceneID, outMD); err != nil {
			return err
		}
		if err := survRepo.Update(m.ctx, tx, m.survivorID, m.survivor); err != nil {
			return err
		}
		outcomeMD = outMD
		aliveAfter = m.survivor.Alive
		recap := buildRecap(outMD)
		_, _ = logRepo.Insert(m.ctx, tx, m.runID, m.survivorID, map[string]any{"turn": prevTurn, "choice": c.Label, "delta": delta}, recap)
		if !aliveAfter {
			deathCause = classifyDeath(m.survivor)
			archRepo := store.NewArchiveRepo(m.db)
			var keySkills []string
			for sk, lvl := range m.survivor.Skills {
				if lvl > 0 {
					keySkills = append(keySkills, string(sk))
				}
				if len(keySkills) == 3 {
					break
				}
			}
			lr := store.NewLogRepo(m.db)
			recent, _ := lr.ListRecent(m.ctx, m.runID, 5)
			var notable []string
			for _, r := range recent {
				notable = append(notable, r.NarrativeRecap)
			}
			cardMD := fmt.Sprintf("# Archive Card\nName: %s\nCause: %s\nDay: %d\nRegion: %s\n", m.survivor.Name, deathCause, m.world.CurrentDay, m.survivor.Region)
			if _, err := archRepo.Insert(m.ctx, tx, m.runID, m.survivorID, m.world.CurrentDay, m.survivor.Region, deathCause, keySkills, notable, nil, m.survivor.Inventory, cardMD); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	m.turn++
	if m.timeline == "" {
		m.timeline = m.sceneRendered + "\n\nOUTCOME:\n" + outcomeMD
	} else {
		m.timeline += "\n\n" + m.sceneRendered + "\n\nOUTCOME:\n" + outcomeMD
	}
	if !aliveAfter {
		m.deaths++
		if err := m.spawnReplacementTx(); err != nil {
			m.md = m.timeline + "\nReplacement failed: " + err.Error()
			return nil
		}
		m.timeline += fmt.Sprintf("\n\n--- Survivor died (%s, deaths: %d). Replacement arrives. ---", deathCause, m.deaths)
		m.md = m.renderSceneLayout()
		return nil
	}
	if err := m.newSceneTx(); err != nil {
		return err
	}
	m.scenesToday++
	if m.scenesToday >= 6 {
		m.world.AdvanceDay()
		m.scenesToday = 0
		m.survivor.SyncEnvironmentDay(m.world.CurrentDay)
		rr := store.NewRunRepo(m.db)
		_ = rr.UpdateDay(m.ctx, nil, m.runID, m.world.CurrentDay)
		m.timeline += "\n\n=== Day advanced to " + fmt.Sprint(m.world.CurrentDay) + " ==="
		_ = m.newSceneTx()
	}
	m.md = m.renderSceneLayout()
	return nil
}

func (m *model) spawnReplacementTx() error {
	return m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		r := engine.RNG(m.world.Seed + int64(m.deaths) + 1)
		newSurv := engine.NewGenericSurvivor(r, m.world.CurrentDay, m.world.OriginSite)
		survRepo := store.NewSurvivorRepo(m.db)
		sid, err := survRepo.Create(m.ctx, m.runID, newSurv)
		if err != nil {
			return err
		}
		m.survivor = newSurv
		m.survivorID = sid
		m.choices = engine.GenerateChoices(m.rng, m.survivor, engine.WithScarcity(m.settings.Scarcity), engine.WithTextDensity(m.settings.TextDensity), engine.WithInfectedPresent(m.survivor.Environment.Infected), engine.WithDifficulty(engine.Difficulty(m.settings.Difficulty)))
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
		m.md = m.renderSceneLayout()
		return nil
	})
}

func (m *model) exportRun() {
	if m.runID == uuid.Nil {
		m.exportStatus = "no-run"
		return
	}
	repo := store.NewSceneRepo(m.db)
	scenes, err := repo.ScenesWithOutcomes(m.ctx, m.runID)
	if err != nil {
		m.exportStatus = "err-collect"
		return
	}
	var b strings.Builder
	b.WriteString("# Zero Point Run Export\nRun: " + m.runID.String() + "\n\n")
	for _, sc := range scenes {
		b.WriteString(fmt.Sprintf("## Day %d Scene\n%s\n", sc.WorldDay, sc.SceneMD))
		if sc.OutcomeMD != "" {
			b.WriteString("\n### Outcome\n" + sc.OutcomeMD + "\n")
		}
		b.WriteString("\n")
	}
	dir := filepath.Join(os.Getenv("HOME"), ".zero-point", "exports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.exportStatus = "err-mkdir"
		return
	}
	fname := fmt.Sprintf("run_%s.md", m.runID.String())
	path := filepath.Join(dir, fname)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		m.exportStatus = "err-write"
		return
	}
	m.exportStatus = path
}

// buildGameView kept for any non-layout fallback (returns scene layout)
func (m *model) buildGameView() string { return m.renderSceneLayout() }

func bar(v int) string {
	width := 10
	fill := int((float64(v)/100.0)*float64(width) + 0.5)
	if fill > width {
		fill = width
	}
	return strings.Repeat("█", fill) + strings.Repeat("·", width-fill)
}
func abbrev(k string) string {
	if len(k) <= 3 {
		return k
	}
	return k[:3]
}

// Helpers reintroduced after refactor ----------------------------------------
func formatCost(c engine.Cost) string {
	parts := []string{fmt.Sprintf("time:%d", c.Time)}
	if c.Fatigue != 0 {
		parts = append(parts, fmt.Sprintf("fatigue:%d", c.Fatigue))
	}
	if c.Hunger != 0 {
		parts = append(parts, fmt.Sprintf("hunger:%d", c.Hunger))
	}
	if c.Thirst != 0 {
		parts = append(parts, fmt.Sprintf("thirst:%d", c.Thirst))
	}
	return strings.Join(parts, ",")
}

func buildRecap(md string) string {
	clean := strings.ReplaceAll(md, "\n", " ")
	clean = strings.TrimSpace(clean)
	if len(clean) > 140 {
		clean = clean[:140] + "..."
	}
	return clean
}

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

func isRuneInput(s string) bool {
	runes := []rune(s)
	return len(runes) == 1 && runes[0] >= 32 && runes[0] < 127
}

// Interactive archive rendering -----------------------------------------------------------
func (m *model) renderArchive() string {
	if len(m.archives) == 0 {
		return "Archive\n(no cards yet)\nEsc to return"
	}
	if m.archiveDetail {
		ac := m.archives[m.archiveIndex]
		var b strings.Builder
		b.WriteString(fmt.Sprintf("ARCHIVE DETAIL (%d/%d)\n", m.archiveIndex+1, len(m.archives)))
		b.WriteString(fmt.Sprintf("Day %d  Region %s  Cause %s\n", ac.WorldDay, ac.Region, ac.CauseOfDeath))
		b.WriteString("Skills: ")
		if len(ac.Skills) == 0 {
			b.WriteString("(none)")
		} else {
			b.WriteString(strings.Join(ac.Skills, ", "))
		}
		b.WriteString("\nRecent: ")
		if len(ac.NotableDecisions) == 0 {
			b.WriteString("(none)")
		} else {
			b.WriteString(strings.Join(ac.NotableDecisions, " | "))
		}
		b.WriteString("\n\nCard:\n")
		b.WriteString(ac.Markdown)
		b.WriteString("\n\nEnter toggle list  Esc back")
		return b.String()
	}
	var b strings.Builder
	b.WriteString("ARCHIVE (Up/Down, Enter view, Esc back)\n")
	for i, ac := range m.archives {
		cursor := "  "
		if i == m.archiveIndex {
			cursor = "> "
		}
		b.WriteString(fmt.Sprintf("%sDay %-3d %-18s %s\n", cursor, ac.WorldDay, ac.Region, ac.CauseOfDeath))
	}
	return b.String()
}

func (m *model) renderHelp() string {
	return "ABOUT / RULES\n\nYou manage sequential survivors in the early outbreak (LAD gate)." +
		" Maintain core needs (health, hunger, thirst, fatigue, morale). Infected risk only after Local Arrival Day." +
		" Each turn: read scene, pick 1 action (1-6) or craft a concise custom verb phrase. Outcomes adjust stats and may cause death." +
		" Death creates an archive card; a new survivor appears immediately.\n\nControls: 1-6 choose | Enter custom | Tab cycle views | L logs | A archive | S settings | E export | F6 LAD debug | Q quit.\n\nEsc returns from subviews."
}

func (m *model) renderSettings() string {
	return fmt.Sprintf("Settings\nScarcity: %v (t toggle)\nDensity: %s (d cycle)\nNarrator: %s (n toggle on/off)\nLanguage: %s (g cycle placeholder)\n", m.settings.Scarcity, m.settings.TextDensity, m.settings.Narrator, m.settings.Language)
}

func (m *model) toggleNarrator() {
	sr := store.NewSettingsRepo(m.db)
	if err := sr.ToggleNarrator(m.ctx, m.runID); err == nil {
		m.refreshSettings()
		m.applyNarratorMode()
	}
}

func (m *model) applyNarratorMode() {
	if m.settings.Narrator == "off" {
		m.narrator = text.NewMinimalFallbackNarrator()
	} else {
		// Re-enable DeepSeek if key present
		key := os.Getenv("DEEPSEEK_API_KEY")
		if key != "" {
			if ds, err := text.NewDeepSeekNarrator(key); err == nil {
				m.narrator = text.WithFallback(ds, text.NewMinimalFallbackNarrator())
			}
		}
	}
}

func (m *model) cycleLanguage() { /* placeholder for future i18n */ }

func (m *model) renderTimeline() string {
	if m.timeline == "" {
		return "TIMELINE\n(no entries yet)\nEsc to return"
	}
	lines := strings.Split(m.timeline, "\n")
	if m.timelineScroll > len(lines)-1 {
		m.timelineScroll = len(lines) - 1
	}
	height := m.height - 2
	if height < 5 {
		height = 5
	}
	start := m.timelineScroll
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	view := strings.Join(lines[start:end], "\n")
	return "TIMELINE (y to toggle, esc to return)\n" + view
}
