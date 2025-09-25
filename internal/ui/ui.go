package ui

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
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
	viewProfile     = "profile_select"
	viewScene       = "scene"
	viewLog         = "log"
	viewArchive     = "archive"
	viewSettings    = "settings"
	viewHelp        = "help"
	viewWorldConfig = "world_config"
	viewTimeline    = "timeline"
	viewLoading     = "loading"
	viewError       = "error"
)

var seedEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

const maxSeedLength = 64

var spinnerFrames = []string{"|", "/", "-", "\\"}

type newGameResultMsg struct {
	result newGameResult
	err    error
}

type spinnerTickMsg struct{}
type tipTickMsg struct{}

type newGameResult struct {
	RunID          uuid.UUID
	SurvivorID     uuid.UUID
	SceneID        uuid.UUID
	RunSeed        engine.RunSeed
	PreRunSeed     engine.RunSeed
	PreRunSeedText string
	World          *engine.World
	Survivor       engine.Survivor
	Choices        []engine.Choice
	CurrentEvent   *engine.EventContext
	SceneRendered  string
	Settings       store.Settings
	Turn           int
	Deaths         int
	ScenesToday    int
	Timeline       string
}

type newGameInput struct {
	SeedText     string
	Scarcity     bool
	Density      string
	Theme        string
	RulesVersion string
	DebugLAD     bool
	ActiveProfile store.Profile
}

func defaultTips() []string {
	return []string{
		"Rotate survivors between exertion and rest to avoid exhaustion penalties.",
		"Scout unknown streets before committing to loud work like barricading.",
		"Use quiet hours ahead of LAD to stash extra water and map safe routes.",
		"Morale drops can spiral; mix in organizing or reflective actions to recover.",
		"Custom actions obey fatigue rules—watch meters before attempting risky moves.",
	}
}

type styleSet struct {
	title          lipgloss.Style
	topBar         lipgloss.Style
	bottomBar      lipgloss.Style
	menuBox        lipgloss.Style
	menuItem       lipgloss.Style
	menuItemActive lipgloss.Style
	scene          lipgloss.Style
	sidebar        lipgloss.Style
	accent         lipgloss.Style
	muted          lipgloss.Style
	borderColor    lipgloss.Color
	barFillColor   lipgloss.Color
	barEmptyColor  lipgloss.Color
	riskLow        lipgloss.Style
	riskModerate   lipgloss.Style
	riskHigh       lipgloss.Style
}

type model struct {
	ctx          context.Context
	world        *engine.World
	survivor     engine.Survivor
	runSeed      engine.RunSeed
	rulesVersion string
	narrator     text.Narrator
	planner      engine.DirectorPlanner
	themeName    string
	palette      palette
	styles       styleSet
	md           string
	// sceneRendered: current scene markdown (no appended choices)
	sceneRendered string
	sceneText     string
	// accumulated timeline (previous scenes+outcomes)
	timeline     string
	choices      []engine.Choice
	currentEvent *engine.EventContext
	// persistence
	db             *store.DB
	runID          uuid.UUID
	survivorID     uuid.UUID
	sceneID        uuid.UUID
	turn           int
	deaths         int
	view           string
	logs           []store.MasterLog
	archives       []store.ArchiveCard
	settings       store.Settings
	profiles       []store.Profile
	activeProfile  store.Profile
	profileIndex   int
	profileEditing bool
	profileInput   string
	profileMessage string
	exportStatus   string
	debugLAD       bool
	tips           []string
	tipIndex       int
	nextTip        time.Time
	loading        bool
	loadingMessage string
	loadingTip     string
	spinnerFrame   int
	startupErr     error
	errorTitle     string
	errorMessage   string
	// custom action
	customInput   string
	customEnabled bool
	customStatus  string
	width         int
	height        int

	// pre-run configuration
	preRunScarcity    bool
	preRunDensity     string
	preRunSeed        engine.RunSeed
	preRunSeedText    string
	preRunSeedBuffer  string
	preRunSeedEditing bool
	preRunTheme       string

	// archive browser
	archiveIndex  int
	archiveDetail bool

	// multi-day loop
	scenesToday int

	// scene scrolling support
	scrollOffset int
	maxScroll    int

	// timeline scrolling
	timelineScroll int
}

func randomSeedText() string {
	buf := make([]byte, 15)
	if _, err := rand.Read(buf); err != nil {
		return "fallback-seed"
	}
	return strings.ToLower(seedEncoding.EncodeToString(buf))
}

func (m *model) choiceStream(label string) *engine.Stream {
	return m.world.Seed.Stream(fmt.Sprintf("day:%d:turn:%d:%s", m.world.CurrentDay, m.turn, label))
}

func (m *model) survivorStream(index int) *engine.Stream {
	return m.runSeed.Stream(fmt.Sprintf("survivor#%d", index))
}

func (m *model) applyTheme(name string) {
	p := paletteFor(name)
	m.themeName = name
	m.palette = p
	m.styles = styleSet{
		title:          lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		topBar:         lipgloss.NewStyle().Background(p.Surface).Foreground(p.Text).Bold(true).Padding(0, 1),
		bottomBar:      lipgloss.NewStyle().Background(p.Surface).Foreground(p.Muted).Padding(0, 1),
		menuBox:        lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(p.Border).Background(p.Surface).Foreground(p.Text).Padding(1, 2),
		menuItem:       lipgloss.NewStyle().Foreground(p.Text),
		menuItemActive: lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		scene:          lipgloss.NewStyle().Foreground(p.Text),
		sidebar:        lipgloss.NewStyle().Foreground(p.Text),
		accent:         lipgloss.NewStyle().Foreground(p.Accent),
		muted:          lipgloss.NewStyle().Foreground(p.Muted),
		borderColor:    p.Border,
		barFillColor:   p.BarFill,
		barEmptyColor:  p.BarEmpty,
		riskLow:        lipgloss.NewStyle().Foreground(p.Success).Bold(true),
		riskModerate:   lipgloss.NewStyle().Foreground(p.Warning).Bold(true),
		riskHigh:       lipgloss.NewStyle().Foreground(p.AccentAlt).Bold(true),
	}
}

func (m *model) newGameCommand(input newGameInput) tea.Cmd {
	ctx := m.ctx
	db := m.db
	narrator := m.narrator
	planner := m.planner
	return func() tea.Msg {
		res, err := runNewGame(ctx, db, narrator, planner, input)
		return newGameResultMsg{result: res, err: err}
	}
}

func (m *model) spinnerTickCmd() tea.Cmd {
	if !m.loading {
		return nil
	}
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func (m *model) tipTickCmd() tea.Cmd {
	if !m.loading || len(m.tips) == 0 {
		return nil
	}
	d := time.Until(m.nextTip)
	if d <= 0 {
		d = 5 * time.Second
	}
	return tea.Tick(d, func(time.Time) tea.Msg { return tipTickMsg{} })
}

func runNewGame(ctx context.Context, db *store.DB, narrator text.Narrator, planner engine.DirectorPlanner, input newGameInput) (newGameResult, error) {
	result := newGameResult{}
	if planner == nil || narrator == nil {
		return result, errors.New("deepseek client unavailable")
	}
	seed, err := engine.NewRunSeed(strings.TrimSpace(input.SeedText))
	if err != nil {
		return result, err
	}
	temp := &model{
		ctx:              ctx,
		db:               db,
		narrator:         narrator,
		planner:          planner,
		rulesVersion:     input.RulesVersion,
		debugLAD:         input.DebugLAD,
		preRunScarcity:   input.Scarcity,
		preRunDensity:    input.Density,
		preRunSeed:       seed,
		preRunSeedText:   strings.TrimSpace(input.SeedText),
		preRunSeedBuffer: strings.TrimSpace(input.SeedText),
		preRunTheme:      input.Theme,
		runSeed:          seed,
		activeProfile:    input.ActiveProfile,
	}
	temp.applyTheme(input.Theme)
	temp.world = engine.NewWorld(seed, input.RulesVersion)
	temp.survivor = engine.NewFirstSurvivor(seed.Stream("survivor#0"), temp.world.OriginSite)
	temp.world.CurrentDay = temp.survivor.Environment.WorldDay
	temp.turn = 0
	temp.deaths = 0
	temp.scenesToday = 0
	temp.timeline = ""
	if err := temp.bootstrapPersistence(); err != nil {
		return result, err
	}
	result = newGameResult{
		RunID:          temp.runID,
		SurvivorID:     temp.survivorID,
		SceneID:        temp.sceneID,
		RunSeed:        temp.runSeed,
		PreRunSeed:     temp.preRunSeed,
		PreRunSeedText: temp.preRunSeedText,
		World:          temp.world,
		Survivor:       temp.survivor,
		Choices:        append([]engine.Choice(nil), temp.choices...),
		CurrentEvent:   temp.currentEvent,
		SceneRendered:  temp.sceneRendered,
		Settings:       temp.settings,
		Turn:           temp.turn,
		Deaths:         temp.deaths,
		ScenesToday:    temp.scenesToday,
		Timeline:       temp.timeline,
	}
	return result, nil
}

func (m *model) loadEventHistory(tx *gorm.DB) (engine.EventHistory, error) {
	repo := store.NewEventRepo(m.db)
	return repo.LoadHistory(m.ctx, tx, m.runID)
}

func (m *model) handleWorldConfigKey(msg tea.KeyMsg) bool {
	k := msg.String()
	if m.preRunSeedEditing {
		switch msg.Type {
		case tea.KeyEnter:
			m.preRunSeedEditing = false
			m.preRunSeedText = strings.TrimSpace(m.preRunSeedBuffer)
			return true
		case tea.KeyEsc:
			m.preRunSeedEditing = false
			m.preRunSeedText = strings.TrimSpace(m.preRunSeedBuffer)
			return true
		case tea.KeyBackspace, tea.KeyCtrlH, tea.KeyDelete:
			if len(m.preRunSeedBuffer) > 0 {
				m.preRunSeedBuffer = m.preRunSeedBuffer[:len(m.preRunSeedBuffer)-1]
				m.preRunSeedText = m.preRunSeedBuffer
			}
			return true
		case tea.KeyRunes:
			for _, r := range msg.Runes {
				if len(m.preRunSeedBuffer) >= maxSeedLength {
					break
				}
				if r >= 32 && r < 127 {
					m.preRunSeedBuffer += string(r)
				}
			}
			m.preRunSeedText = strings.TrimSpace(m.preRunSeedBuffer)
			return true
		}
		return true
	}

	switch k {
	case "enter":
		m.preRunSeedEditing = true
		return true
	case "1":
		m.preRunScarcity = !m.preRunScarcity
		return true
	case "2":
		m.preRunDensity = cycleDensityLocal(m.preRunDensity)
		return true
	case "3":
		next := nextThemeName(m.themeName, 1)
		m.preRunTheme = next
		m.applyTheme(next)
		return true
	case "esc":
		m.view = viewMainMenu
		return true
	}
	return false
}

// initialModel boots to main menu; game state seeded but not persisted until New Game selected.
func initialModel(ctx context.Context, db *store.DB, narrator text.Narrator, planner engine.DirectorPlanner, cfg util.Config, startupErr error) model {
	runSeed, err := engine.NewRunSeed(cfg.SeedText)
	if err != nil {
		runSeed, _ = engine.NewRunSeed("fallback-seed")
	}
	w := engine.NewWorld(runSeed, cfg.RulesVersion)
	s := engine.NewFirstSurvivor(runSeed.Stream("survivor#0"), w.OriginSite)
	w.CurrentDay = s.Environment.WorldDay
	m := model{
		ctx:          ctx,
		world:        w,
		survivor:     s,
		runSeed:      runSeed,
		rulesVersion: cfg.RulesVersion,
		narrator:     narrator,
		planner:      planner,
		db:           db,
		debugLAD:     cfg.DebugLAD,
	}
	m.applyTheme("catppuccin")
	profRepo := store.NewProfileRepo(db)
	profiles, err := profRepo.List(ctx)
	if err != nil {
		m.errorTitle = "Profile Error"
		m.errorMessage = err.Error()
		m.view = viewError
		return m
	}
	if len(profiles) == 0 {
		if created, err := profRepo.Create(ctx, "main"); err == nil {
			profiles = append(profiles, created)
		}
	}
	m.profiles = profiles
	if len(m.profiles) == 0 {
		m.errorTitle = "Profile Error"
		m.errorMessage = "Unable to initialize profile storage."
		m.view = viewError
		return m
	}
	m.profileIndex = 0
	m.activeProfile = m.profiles[0] // Set default active profile
	m.profileEditing = false
	m.profileInput = ""
	// If we have profiles, go directly to main menu instead of profile selection
	if len(m.profiles) > 0 {
		m.view = viewMainMenu
	} else {
		m.profileMessage = "Press Enter to use profile, N to create a new one."
		m.view = viewProfile
	}
	m.preRunScarcity = false
	m.preRunDensity = cfg.TextDensity
	if m.preRunDensity == "" {
		m.preRunDensity = "standard"
	}
	m.preRunSeed = runSeed
	m.preRunSeedText = runSeed.Text
	m.preRunSeedBuffer = runSeed.Text
	m.preRunTheme = m.themeName
	tipRepo := store.NewTipRepo(db)
	if tipRecords, err := tipRepo.All(ctx); err == nil && len(tipRecords) > 0 {
		for _, tip := range tipRecords {
			m.tips = append(m.tips, tip.Text)
		}
	} else {
		m.tips = defaultTips()
	}
	if len(m.tips) > 0 {
		m.loadingTip = m.tips[0]
	}
	m.tipIndex = 0
	m.startupErr = startupErr
	if startupErr != nil {
		m.errorTitle = "Startup Error"
		m.errorMessage = startupErr.Error()
		m.view = viewError
	}
	return m
}

// bootstrapPersistence creates run, survivor, initial scene & choices.
func (m *model) bootstrapPersistence() error {
	runRepo := store.NewRunRepo(m.db)
	survRepo := store.NewSurvivorRepo(m.db)
	run, err := runRepo.CreateWithSeed(m.ctx, m.activeProfile.ID, m.world.OriginSite, m.preRunSeedText, m.rulesVersion)
	if err != nil {
		return err
	}
	m.runID = run.ID
	_ = store.NewProfileRepo(m.db).Touch(m.ctx, m.activeProfile.ID)
	// Mix runID and rules into runSeed for per-run uniqueness (post-persist)
	m.runSeed = m.runSeed.WithRunContext(m.runID.String(), m.rulesVersion)
	// Ensure world streams also use the mixed seed
	if m.world != nil {
		m.world.Seed = m.runSeed
	}
	sid, err := survRepo.Create(m.ctx, m.runID, m.survivor)
	if err != nil {
		return err
	}
	m.survivorID = sid
	if err := m.newSceneTx(); err != nil {
		return err
	}
	setRepo := store.NewSettingsRepo(m.db)
	_ = setRepo.Upsert(m.ctx, m.runID, m.preRunScarcity, m.preRunDensity, "en", "auto", "standard", m.preRunTheme)
	m.refreshSettings()
	return nil
}

// startNewGame persists and enters scene view.
func (m *model) startNewGame() tea.Cmd {
	if m.activeProfile.ID == uuid.Nil {
		m.view = viewProfile
		m.profileMessage = "Select or create a profile before starting."
		return nil
	}
	if m.planner == nil || m.narrator == nil {
		m.view = viewError
		m.errorTitle = "DeepSeek Required"
		m.errorMessage = "Configure DEEPSEEK_API_KEY before starting a new game."
		return nil
	}
	input := strings.TrimSpace(m.preRunSeedBuffer)
	if input == "" {
		m.md = "Seed cannot be empty"
		return nil
	}
	if _, err := engine.NewRunSeed(input); err != nil {
		m.md = "Invalid seed string"
		return nil
	}
	m.preRunSeedText = input
	m.preRunSeedBuffer = input
	m.loading = true
	m.loadingMessage = "Preparing new survivor..."
	m.spinnerFrame = 0
	m.view = viewLoading
	if len(m.tips) == 0 {
		m.tips = defaultTips()
	}
	if len(m.tips) > 0 {
		m.tipIndex = (m.tipIndex + 1) % len(m.tips)
		m.loadingTip = m.tips[m.tipIndex]
		m.nextTip = time.Now().Add(5 * time.Second)
	}
	inputCfg := newGameInput{
		SeedText:      input,
		Scarcity:      m.preRunScarcity,
		Density:       m.preRunDensity,
		Theme:         m.preRunTheme,
		RulesVersion:  m.rulesVersion,
		DebugLAD:      m.debugLAD,
		ActiveProfile: m.activeProfile,
	}
	return tea.Batch(m.newGameCommand(inputCfg), m.spinnerTickCmd(), m.tipTickCmd())
}

// continueGame loads latest run & alive survivor, then generates a fresh scene (simplified resume).
func (m *model) continueGame() {
	m.loading = false
	if m.activeProfile.ID == uuid.Nil {
		m.view = viewProfile
		m.profileMessage = "Select a profile before continuing."
		return
	}
	if m.planner == nil || m.narrator == nil {
		m.view = viewError
		m.errorTitle = "DeepSeek Required"
		m.errorMessage = "Configure DEEPSEEK_API_KEY before continuing a run."
		return
	}
	rr := store.NewRunRepo(m.db)
	run, err := rr.GetLatestRun(m.ctx, m.activeProfile.ID)
	if err != nil {
		m.md = "No run to continue"
		return
	}
	runSeed, err := engine.NewRunSeed(run.SeedText)
	if err != nil {
		m.md = "Run seed invalid"
		return
	}
	// Mix in persisted runID and rules for this resumed run
	runSeed = runSeed.WithRunContext(run.ID.String(), run.RulesVersion)
	m.preRunSeedText = run.SeedText
	m.preRunSeed = runSeed
	m.runSeed = runSeed
	m.rulesVersion = run.RulesVersion
	m.preRunSeedBuffer = run.SeedText
	m.world = &engine.World{
		OriginSite:   run.OriginSite,
		Seed:         runSeed,
		RulesVersion: m.rulesVersion,
		CurrentDay:   run.CurrentDay,
	}
	sr := store.NewSurvivorRepo(m.db)
	surv, sid, err := sr.GetAliveSurvivor(m.ctx, run.ID)
	if err != nil {
		m.md = "No alive survivor"
		return
	}
	m.survivor = surv
	m.survivorID = sid
	m.runID = run.ID
	m.turn = 0
	m.deaths = 0
	m.scenesToday = 0
	m.refreshSettings()
	m.preRunTheme = m.settings.Theme
	if m.preRunTheme != "" {
		m.applyTheme(m.preRunTheme)
	}
	if err := m.newSceneTx(); err != nil {
		m.md = "Failed to build scene: " + err.Error()
		return
	}
	_ = store.NewProfileRepo(m.db).Touch(m.ctx, m.activeProfile.ID)
	m.view = viewScene
}

// newSceneTx inserts scene + choices transactionally, renders markdown.
func (m *model) newSceneTx() error {
	return m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		history, err := m.loadEventHistory(tx)
		if err != nil {
			return err
		}
		choices, eventCtx, err := engine.GenerateChoices(m.ctx, m.planner, m.choiceStream("choices"), &m.survivor, history, m.turn, engine.WithScarcity(m.settings.Scarcity), engine.WithTextDensity(m.settings.TextDensity), engine.WithInfectedPresent(m.survivor.Environment.Infected), engine.WithDifficulty(engine.Difficulty(m.settings.Difficulty)))
		if err != nil {
			return err
		}
		m.choices = choices
		m.currentEvent = eventCtx
		m.customEnabled = true
		m.customStatus = ""
		// cooldown check will happen in handleCustomAction
		state := m.survivor.NarrativeState()
		cacheRepo := store.NewNarrationCacheRepo(m.db)
		var md string
		var sceneHash []byte
		if h, err := text.SceneCacheKey(state); err == nil {
			sceneHash = h
			if cached, ok, err := cacheRepo.Get(m.ctx, tx, m.runID, "scene", h); err == nil && ok {
				md = cached
			}
		}
		if md == "" {
			generated, err := m.narrator.Scene(m.ctx, state)
			if err != nil {
				return err
			}
			md = generated
			if sceneHash != nil {
				_ = cacheRepo.Put(m.ctx, tx, m.runID, "scene", sceneHash, md)
			}
		}
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
	if m.view == viewProfile {
		return m.renderProfileSelect()
	}
	if m.view == viewMainMenu {
		return m.renderMainMenu()
	}
	if m.view == viewWorldConfig {
		return m.renderWorldConfig()
	}
	if m.view == viewLoading {
		return m.renderLoading()
	}
	if m.view == viewError {
		return m.renderErrorScreen()
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
	case newGameResultMsg:
		m.loading = false
		if msg.err != nil {
			m.loadingMessage = ""
			m.loadingTip = ""
			m.errorTitle = "New Game Failed"
			m.errorMessage = msg.err.Error()
			m.view = viewError
			return m, nil
		}
		m.errorTitle = ""
		m.errorMessage = ""
		res := msg.result
		m.runID = res.RunID
		m.survivorID = res.SurvivorID
		m.sceneID = res.SceneID
		m.runSeed = res.RunSeed
		m.preRunSeed = res.PreRunSeed
		m.preRunSeedText = res.PreRunSeedText
		m.preRunSeedBuffer = res.PreRunSeedText
		m.world = res.World
		m.survivor = res.Survivor
		m.choices = res.Choices
		m.currentEvent = res.CurrentEvent
		m.sceneRendered = res.SceneRendered
		m.settings = res.Settings
		m.applyTheme(res.Settings.Theme)
		m.preRunTheme = res.Settings.Theme
		m.preRunScarcity = res.Settings.Scarcity
		m.preRunDensity = res.Settings.TextDensity
		m.turn = res.Turn
		m.deaths = res.Deaths
		m.scenesToday = res.ScenesToday
		m.timeline = res.Timeline
		m.view = viewScene
		m.loadingMessage = ""
		m.loadingTip = ""
		m.md = m.renderSceneLayout()
		return m, nil
	case spinnerTickMsg:
		if m.loading {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, m.spinnerTickCmd()
		}
		return m, nil
	case tipTickMsg:
		if m.loading && len(m.tips) > 0 {
			m.tipIndex = (m.tipIndex + 1) % len(m.tips)
			m.loadingTip = m.tips[m.tipIndex]
			m.nextTip = time.Now().Add(5 * time.Second)
			return m, m.tipTickCmd()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.view == viewScene {
			m.md = m.renderSceneLayout()
		}
		return m, nil
	case tea.KeyMsg:
		k := msg.String()
		if k == "ctrl+c" || (k == "q" && !m.preRunSeedEditing) {
			return m, tea.Quit
		}
		if m.view == viewProfile {
			if m.profileEditing {
				switch msg.Type {
				case tea.KeyEnter:
					name := strings.TrimSpace(m.profileInput)
					if name == "" {
						m.profileMessage = "Profile name cannot be empty."
						return m, nil
					}
					profRepo := store.NewProfileRepo(m.db)
					if _, err := profRepo.Create(m.ctx, name); err != nil {
						m.profileMessage = err.Error()
						return m, nil
					}
					if profiles, err := profRepo.List(m.ctx); err == nil {
						m.profiles = profiles
						m.profileIndex = 0
					}
					m.profileEditing = false
					m.profileInput = ""
					m.profileMessage = "Profile created. Press Enter to continue."
					return m, nil
				case tea.KeyEsc:
					m.profileEditing = false
					m.profileInput = ""
					m.profileMessage = "Press Enter to use profile, N to create a new one."
					return m, nil
				case tea.KeyBackspace, tea.KeyCtrlH, tea.KeyDelete:
					if len(m.profileInput) > 0 {
						m.profileInput = m.profileInput[:len(m.profileInput)-1]
					}
					return m, nil
				case tea.KeyRunes:
					for _, r := range msg.Runes {
						if r >= 32 && r < 127 {
							m.profileInput += string(r)
						}
					}
					return m, nil
				}
				return m, nil
			}
			switch k {
			case "up", "k":
				if m.profileIndex > 0 {
					m.profileIndex--
				}
			case "down", "j":
				if m.profileIndex < len(m.profiles)-1 {
					m.profileIndex++
				}
			case "n":
				m.profileEditing = true
				m.profileInput = ""
				m.profileMessage = "Type a profile name. Enter to create, Esc to cancel."
			case "enter":
				if len(m.profiles) == 0 {
					return m, nil
				}
				selected := m.profiles[m.profileIndex]
				m.activeProfile = selected
				_ = store.NewProfileRepo(m.db).Touch(m.ctx, selected.ID)
				m.view = viewMainMenu
				m.profileMessage = ""
				return m, nil
			}
			return m, nil
		}
		if m.view == viewError {
			switch k {
			case "enter", "esc", " ":
				m.view = viewMainMenu
			}
			return m, nil
		}
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
		if k == "esc" {
			switch m.view {
			case viewHelp, viewSettings:
				m.view = viewScene
				return m, nil
			case viewArchive:
				m.view = viewScene
				return m, nil
			case viewWorldConfig:
				m.view = viewMainMenu
				return m, nil
			}
		}
		if m.view == viewMainMenu {
			switch k {
			case "1":
				if cmd := m.startNewGame(); cmd != nil {
					return m, cmd
				}
			case "2":
				m.continueGame()
			case "3":
				m.view = viewWorldConfig
			case "5":
				m.view = viewHelp
			case "p":
				m.view = viewProfile
			}
			return m, nil
		}
		if m.view == viewWorldConfig {
			if m.handleWorldConfigKey(msg) {
				if !m.preRunSeedEditing {
					m.preRunSeedText = strings.TrimSpace(m.preRunSeedBuffer)
				}
				m.md = m.renderWorldConfig()
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
			case "g":
				m.cycleLanguage()
			case "c":
				m.cycleTheme()
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
		case "m":
			m.view = viewMainMenu
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
				m.scrollOffset += 6
			case "pgup", "ctrl+b":
				m.scrollOffset -= 6
			case "home":
				m.scrollOffset = 0
			case "end":
				m.scrollOffset = m.maxScroll
			default:
				goto doneSceneScroll
			}
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			if m.scrollOffset > m.maxScroll {
				m.scrollOffset = m.maxScroll
			}
			m.md = m.renderSceneLayout()
			return m, nil
		}
	doneSceneScroll:
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
	scenePanel := lipgloss.JoinVertical(lipgloss.Left,
		m.renderCharacterSummary(mainWidth-2),
		"",
		m.renderSceneNarrative(mainWidth-2),
		"",
		m.renderChoicesSection(mainWidth-2),
	)
	sceneLines := strings.Split(scenePanel, "\n")
	availHeight := m.height - 4
	if availHeight < 5 {
		availHeight = len(sceneLines)
	}
	if availHeight > 0 && len(sceneLines) > availHeight {
		m.maxScroll = len(sceneLines) - availHeight
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		if m.scrollOffset > m.maxScroll {
			m.scrollOffset = m.maxScroll
		}
		sceneLines = sceneLines[m.scrollOffset : m.scrollOffset+availHeight]
	} else {
		m.scrollOffset = 0
		m.maxScroll = 0
	}
	scenePanel = strings.Join(sceneLines, "\n")
	main := m.styles.scene.Copy().Width(mainWidth).Render(scenePanel)
	sideSections := []string{
		m.renderStatsSection(sidebarWidth - 2),
		m.renderSkillsSection(sidebarWidth - 2),
		m.renderConditionsSection(sidebarWidth - 2),
		m.renderInventorySection(sidebarWidth - 2),
	}
	if meters := m.renderMetersSection(sidebarWidth - 2); meters != "" {
		sideSections = append(sideSections, meters)
	}
	side := m.styles.sidebar.Copy().Width(sidebarWidth).Render(strings.Join(sideSections, "\n\n"))
	body := lipgloss.JoinHorizontal(lipgloss.Top, main, side)
	bottom := m.renderBottomBar()
	return lipgloss.JoinVertical(lipgloss.Left, top, body, bottom)
}

func (m *model) renderLoading() string {
	width := m.width
	if width < 50 {
		width = 50
	}
	height := m.height
	if height < 10 {
		height = 10
	}
	spinner := ""
	if len(spinnerFrames) > 0 {
		spinner = spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
	}
	message := m.loadingMessage
	if message == "" {
		message = "Loading..."
	}
	tip := m.loadingTip
	if tip == "" && len(m.tips) > 0 {
		tip = m.tips[m.tipIndex%len(m.tips)]
	}
	inner := fmt.Sprintf("%s %s\n\nTip: %s", m.styles.accent.Render(spinner), message, tip)
	box := m.styles.menuBox.Width(width / 2).Render(inner)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m *model) renderErrorScreen() string {
	width := m.width
	if width < 60 {
		width = 60
	}
	height := m.height
	if height < 12 {
		height = 12
	}
	title := m.errorTitle
	if title == "" {
		title = "DeepSeek Required"
	}
	msg := m.errorMessage
	if strings.TrimSpace(msg) == "" {
		msg = "Set DEEPSEEK_API_KEY in your environment to enable narration and events."
	}
	msg = msg + "\n\nPress Enter to return to the main menu."
	body := lipgloss.JoinVertical(lipgloss.Left,
		m.styles.title.Render(title),
		m.styles.muted.Render(msg),
	)
	box := m.styles.menuBox.Width(56).Render(body)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m *model) sectionBox(title, body string, width int) string {
	header := m.styles.title.Render(title)
	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	return lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(m.styles.borderColor).Padding(0, 1).Width(width).Render(content)
}

func (m *model) renderCharacterSummary(width int) string {
	s := m.survivor
	env := s.Environment
	lines := []string{
		fmt.Sprintf("%s (%s, %d)", s.Name, strings.Title(s.Background), s.Age),
		fmt.Sprintf("Region: %s • Location: %s", s.Region, strings.Title(string(s.Location))),
		fmt.Sprintf("Group: %s (%d)", strings.Title(string(s.Group)), s.GroupSize),
		fmt.Sprintf("Day %d • LAD %d • %s", env.WorldDay, env.LAD, strings.Title(env.TimeOfDay)),
		fmt.Sprintf("Weather: %s • Season: %s", strings.Title(string(env.Weather)), strings.Title(string(env.Season))),
	}
	return m.sectionBox("Character", strings.Join(lines, "\n"), width)
}

func (m *model) renderSceneNarrative(width int) string {
	body := strings.TrimSpace(m.sceneRendered)
	return m.sectionBox("Scene", body, width)
}

func (m *model) renderChoicesSection(width int) string {
	if len(m.choices) == 0 {
		return m.sectionBox("Choices", "No actions available", width)
	}
	var b strings.Builder
	for i, choice := range m.choices {
		idx := i + 1
		arch := choice.Archetype
		if arch == "" {
			arch = "action"
		}
		line1 := fmt.Sprintf("%d. %s — %s", idx, strings.Title(arch), choice.Label)
		wrapped := lipgloss.NewStyle().Width(width - 2).Render(line1)
		riskStyle := m.riskStyle(choice.Risk)
		riskLine := fmt.Sprintf("Risk: %s", riskStyle.Render(strings.ToUpper(string(choice.Risk))))
		costLine := fmt.Sprintf("Cost: %s", formatCost(choice.Cost))
		line2 := fmt.Sprintf("   %s   %s", costLine, riskLine)
		b.WriteString(wrapped + "\n")
		b.WriteString(m.styles.muted.Render(line2))
		if i < len(m.choices)-1 {
			b.WriteString("\n\n")
		}
	}
	if m.customStatus != "" {
		b.WriteString("\n\n" + m.styles.muted.Render("Custom: "+m.customStatus))
	}
	return m.sectionBox("Choices", b.String(), width)
}

func (m *model) renderStatsSection(width int) string {
	s := m.survivor.Stats
	lines := []string{
		fmt.Sprintf("Health  %s %3d", m.renderBar(s.Health), s.Health),
		fmt.Sprintf("Hunger  %s %3d", m.renderBar(s.Hunger), s.Hunger),
		fmt.Sprintf("Thirst  %s %3d", m.renderBar(s.Thirst), s.Thirst),
		fmt.Sprintf("Fatigue %s %3d", m.renderBar(s.Fatigue), s.Fatigue),
		fmt.Sprintf("Morale  %s %3d", m.renderBar(s.Morale), s.Morale),
	}
	return m.sectionBox("Stats", strings.Join(lines, "\n"), width)
}

func (m *model) renderBar(value int) string {
	barWidth := 18
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	filled := value * barWidth / 100
	if value == 100 {
		filled = barWidth
	}
	fill := strings.Repeat("█", filled)
	empty := strings.Repeat("░", barWidth-filled)
	fillStyle := lipgloss.NewStyle().Foreground(m.styles.barFillColor)
	emptyStyle := lipgloss.NewStyle().Foreground(m.styles.barEmptyColor)
	return fillStyle.Render(fill) + emptyStyle.Render(empty)
}

func (m *model) renderSkillsSection(width int) string {
	if len(m.survivor.Skills) == 0 {
		return m.sectionBox("Skills", "No recorded skills", width)
	}
	type entry struct {
		name string
		lvl  int
	}
	var entries []entry
	for skill, lvl := range m.survivor.Skills {
		entries = append(entries, entry{name: readableSkill(skill), lvl: lvl})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%s: %d", e.name, e.lvl))
	}
	return m.sectionBox("Skills", strings.Join(lines, "\n"), width)
}

func (m *model) renderConditionsSection(width int) string {
	if len(m.survivor.Conditions) == 0 {
		return m.sectionBox("Conditions", "None", width)
	}
	var lines []string
	for _, c := range m.survivor.Conditions {
		lines = append(lines, strings.Title(string(c)))
	}
	return m.sectionBox("Conditions", strings.Join(lines, "\n"), width)
}

func (m *model) renderInventorySection(width int) string {
	inv := m.survivor.Inventory
	lines := []string{
		fmt.Sprintf("Food: %.1fd • Water: %.1fL", inv.FoodDays, inv.WaterLiters),
		"Weapons: " + formatList(inv.Weapons),
		"Medical: " + formatList(inv.Medical),
		"Tools: " + formatList(inv.Tools),
		"Special: " + formatList(inv.Special),
	}
	if inv.Memento != "" {
		lines = append(lines, "Memento: "+inv.Memento)
	}
	return m.sectionBox("Inventory", strings.Join(lines, "\n"), width)
}

func (m *model) renderMetersSection(width int) string {
	if len(m.survivor.Meters) == 0 {
		return ""
	}
	var lines []string
	keys := make([]string, 0, len(m.survivor.Meters))
	for k := range m.survivor.Meters {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, key := range keys {
		val := m.survivor.Meters[engine.Meter(key)]
		lines = append(lines, fmt.Sprintf("%s %s %3d", strings.Title(key), m.renderBar(val), val))
	}
	return m.sectionBox("Meters", strings.Join(lines, "\n"), width)
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}

func readableSkill(sk engine.Skill) string {
	return strings.Title(strings.ReplaceAll(string(sk), "_", " "))
}

func (m *model) riskStyle(level engine.RiskLevel) lipgloss.Style {
	switch level {
	case engine.RiskLow:
		return m.styles.riskLow
	case engine.RiskModerate:
		return m.styles.riskModerate
	default:
		return m.styles.riskHigh
	}
}

func (m *model) renderTopBar() string {
	// Left: ZERO POINT • Region • Local Date & Time TZ • Season • Temp Band
	// Right: World Day and optional [LAD:n] when debug
	leftParts := []string{
		"ZERO POINT",
		m.survivor.Region,
	}
	// Local time string
	locStr := engine.NarrativeLocalTime(m.survivor)
	leftParts = append(leftParts, locStr)
	leftParts = append(leftParts, string(m.survivor.Environment.Season))
	leftParts = append(leftParts, string(m.survivor.Environment.TempBand))
	left := strings.Join(leftParts, " • ")
	right := fmt.Sprintf("Day %d", m.survivor.Environment.WorldDay)
	if m.debugLAD {
		right += fmt.Sprintf("  [LAD:%d]", m.survivor.Environment.LAD)
	}
	w := m.width
	if w <= 0 {
		w = 100
	}
	gap := w - len(left) - len(right)
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + right
	return m.styles.topBar.Render(bar)
}

func (m *model) renderBottomBar() string {
	left := "[1-6] choose  [Enter] commit custom  [Tab] cycle  [L] log  [A] archive  [S] settings  [M] menu  [?] help  [Q] quit"
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
	return m.styles.bottomBar.Render(left + "\n" + line)
}

// Main menu rendering.
func (m *model) renderMainMenu() string {
	width := m.width
	if width < 50 {
		width = 50
	}
	height := m.height
	if height < 12 {
		height = 12
	}

	header := m.styles.title.Render("ZERO POINT — MAIN MENU")
	options := []string{
		"Profile: " + func() string {
			if m.activeProfile.ID == uuid.Nil {
				return "<select profile>"
			}
			return m.activeProfile.Name
		}(),
		"",
		"[1] New Game",
		"[2] Continue Game",
		"[3] World Settings",
		"[4] Survivor Archive",
		"[5] About / Rules",
		"",
		"P Switch Profile",
		"Q Quit",
	}
	if m.startupErr != nil {
		msg := m.errorMessage
		if msg == "" {
			msg = "DeepSeek API key missing."
		}
		options = append(options, "", m.styles.muted.Render(msg))
	}
	body := strings.Join(options, "\n")
	box := m.styles.menuBox.Width(46).Render(lipgloss.JoinVertical(lipgloss.Left, header, body))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m *model) renderWorldConfig() string {
	width := m.width
	if width < 68 {
		width = 68
	}
	height := m.height
	if height < 14 {
		height = 14
	}
	seedHint := "Press Enter to edit (Esc to cancel)."
	if m.preRunSeedEditing {
		seedHint = "Editing — Enter to confirm, Esc to cancel."
	}
	scar := "Off"
	if m.preRunScarcity {
		scar = "On"
	}
	lines := []string{
		"WORLD SETTINGS (Pre-Run)",
		"",
		fmt.Sprintf("Seed [%d/%d]", len(m.preRunSeedBuffer), maxSeedLength),
		m.styles.accent.Render(m.preRunSeedBuffer),
		m.styles.muted.Render(seedHint),
		"",
		fmt.Sprintf("Scarcity: %s %s", scar, m.styles.muted.Render("(press 1 to toggle)")),
		fmt.Sprintf("Text Density: %s %s", m.preRunDensity, m.styles.muted.Render("(press 2 to cycle)")),
		fmt.Sprintf("Theme: %s %s", m.preRunTheme, m.styles.muted.Render("(press 3 to cycle)")),
		"",
		m.styles.muted.Render("Use Backspace while editing. Press Esc to return."),
	}
	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	box := m.styles.menuBox.Width(66).Render(body)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
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
		if s.Theme != "" {
			m.applyTheme(s.Theme)
		}
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
	history, err := m.loadEventHistory(nil)
	if err != nil {
		m.md = "Choice generation failed: " + err.Error()
		return
	}
	choices, eventCtx, err := engine.GenerateChoices(m.ctx, m.planner, m.choiceStream("choices"), &m.survivor, history, m.turn, engine.WithScarcity(m.settings.Scarcity), engine.WithTextDensity(m.settings.TextDensity), engine.WithInfectedPresent(m.survivor.Environment.Infected), engine.WithDifficulty(engine.Difficulty(m.settings.Difficulty)))
	if err != nil {
		m.md = "Choice generation failed: " + err.Error()
		return
	}
	m.choices = choices
	m.currentEvent = eventCtx
	m.md = m.renderSceneLayout()
}

func (m *model) handleCustomAction() {
	// enforce 2-scene cooldown since last custom (stored turn number)
	if m.survivor.Meters != nil {
		if last, ok := m.survivor.Meters[engine.MeterCustomLastTurn]; ok {
			if m.turn-last < 2 { // need at least 2 full scenes gap
				m.customStatus = "Custom action unavailable now"
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
	if err := m.resolveChoiceTx(choice); err != nil {
		m.md = "Resolution failed: " + err.Error()
		return
	}
	m.customInput = ""
	m.customStatus = ""
}

// resolveChoiceTx (replacing earlier version with new layout integration)
func (m *model) resolveChoiceTx(c engine.Choice) error {
	var aliveAfter bool
	var outcomeMD string
	prevTurn := m.turn
	var deathCause string
	eventCtx := m.currentEvent
	m.currentEvent = nil
	eventRepo := store.NewEventRepo(m.db)
	cacheRepo := store.NewNarrationCacheRepo(m.db)
	err := m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		deltaStream := m.choiceStream("delta").Child(fmt.Sprintf("choice:%s", c.ID))
		res := engine.ApplyChoice(&m.survivor, c, engine.Difficulty(m.settings.Difficulty), m.turn, deltaStream)
		upRepo := store.NewUpdateRepo(m.db)
		outRepo := store.NewOutcomeRepo(m.db)
		survRepo := store.NewSurvivorRepo(m.db)
		logRepo := store.NewLogRepo(m.db)
		if _, err := upRepo.Insert(m.ctx, tx, m.sceneID, res.Delta, res.Added, res.Removed); err != nil {
			return err
		}
		stateAfter := m.survivor.NarrativeState()
		var outMD string
		var outcomeHash []byte
		if h, err := text.OutcomeCacheKey(stateAfter, c, res.Delta); err == nil {
			outcomeHash = h
			if cached, ok, err := cacheRepo.Get(m.ctx, tx, m.runID, "outcome", h); err == nil && ok {
				outMD = cached
			}
		}
		if outMD == "" {
			generated, err := m.narrator.Outcome(m.ctx, stateAfter, c, res.Delta)
			if err != nil {
				return err
			}
			outMD = generated
			if outcomeHash != nil {
				_ = cacheRepo.Put(m.ctx, tx, m.runID, "outcome", outcomeHash, outMD)
			}
		}
		if _, err := outRepo.Insert(m.ctx, tx, m.sceneID, outMD); err != nil {
			return err
		}
		if err := survRepo.Update(m.ctx, tx, m.survivorID, m.survivor); err != nil {
			return err
		}
		outcomeMD = outMD
		aliveAfter = m.survivor.Alive
		recap := buildRecap(outMD)
		_, _ = logRepo.Insert(m.ctx, tx, m.runID, m.survivorID, map[string]any{
			"turn":               prevTurn,
			"choice":             c.Label,
			"delta":              res.Delta,
			"conditions_added":   res.Added,
			"conditions_removed": res.Removed,
		}, recap)
		if eventCtx != nil {
			cooldown := prevTurn + eventCtx.Event.CooldownScenes + 1
			rec := store.EventInstanceRecord{
				RunID:              m.runID,
				SurvivorID:         m.survivorID,
				EventID:            eventCtx.Event.ID,
				WorldDay:           m.world.CurrentDay,
				SceneIdx:           prevTurn,
				CooldownUntilScene: cooldown,
				OnceFired:          eventCtx.Event.OncePerRun,
			}
			if err := eventRepo.Insert(m.ctx, tx, rec); err != nil {
				return err
			}
		}
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
		if err := m.newSceneTx(); err != nil {
			return err
		}
	}
	m.md = m.renderSceneLayout()
	return nil
}

func (m *model) spawnReplacementTx() error {
	return m.db.WithTx(m.ctx, func(tx *gorm.DB) error {
		newSurv := engine.NewGenericSurvivor(m.survivorStream(m.deaths+1), m.world.CurrentDay, m.world.OriginSite)
		survRepo := store.NewSurvivorRepo(m.db)
		sid, err := survRepo.Create(m.ctx, m.runID, newSurv)
		if err != nil {
			return err
		}
		m.survivor = newSurv
		m.survivorID = sid
		history, err := m.loadEventHistory(tx)
		if err != nil {
			return err
		}
		choices, eventCtx, err := engine.GenerateChoices(m.ctx, m.planner, m.choiceStream("choices"), &m.survivor, history, m.turn, engine.WithScarcity(m.settings.Scarcity), engine.WithTextDensity(m.settings.TextDensity), engine.WithInfectedPresent(m.survivor.Environment.Infected), engine.WithDifficulty(engine.Difficulty(m.settings.Difficulty)))
		if err != nil {
			return err
		}
		m.choices = choices
		m.currentEvent = eventCtx
		state := m.survivor.NarrativeState()
		cacheRepo := store.NewNarrationCacheRepo(m.db)
		var md string
		var sceneHash []byte
		if h, err := text.SceneCacheKey(state); err == nil {
			sceneHash = h
			if cached, ok, err := cacheRepo.Get(m.ctx, tx, m.runID, "scene", h); err == nil && ok {
				md = cached
			}
		}
		if md == "" {
			generated, err := m.narrator.Scene(m.ctx, state)
			if err != nil {
				return err
			}
			md = generated
			if sceneHash != nil {
				_ = cacheRepo.Put(m.ctx, tx, m.runID, "scene", sceneHash, md)
			}
		}
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
	if err := os.MkdirAll(dir, 0o700); err != nil {
		m.exportStatus = "err-mkdir"
		return
	}
	fname := fmt.Sprintf("run_%s.md", m.runID.String())
	path := filepath.Join(dir, fname)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		m.exportStatus = "err-write"
		return
	}
	m.exportStatus = path
}

// buildGameView kept for any non-layout fallback (returns scene layout)
func (m *model) buildGameView() string { return m.renderSceneLayout() }

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
	width := m.width
	if width < 76 {
		width = 76
	}
	height := m.height
	if height < 18 {
		height = 18
	}
	lines := []string{
		"ABOUT / RULES",
		"",
		fmt.Sprintf("Seed & Rules: %s • %s", m.runSeed.Text, m.rulesVersion),
		"",
		"Zero Point is a sequential survival anthology directed by DeepSeek.",
		m.styles.muted.Render("Maintain health, hunger, thirst, fatigue, and morale."),
		m.styles.muted.Render("Infected threats appear only after Local Arrival Day (LAD)."),
		m.styles.muted.Render("Choose one action each turn (1-6) or enter a concise custom verb."),
		m.styles.muted.Render("Outcomes adjust stats; death archives the survivor and the timeline continues."),
		"",
		"Controls:",
		m.styles.muted.Render("1-6 choose action   Enter commit custom input   Tab cycle views"),
		m.styles.muted.Render("L logs   A archive   S settings   Y timeline   E export run"),
		m.styles.muted.Render("T scarcity   D density   C theme   F6 toggle LAD debug"),
		m.styles.muted.Render("Press ? or Esc to return."),
	}
	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	box := m.styles.menuBox.Width(72).Render(body)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m *model) renderSettings() string {
	return fmt.Sprintf("Settings\nSeed & Rules: %s • %s\nScarcity: %v (t toggle)\nDensity: %s (d cycle)\nTheme: %s (c cycle)\nLanguage: %s (g cycle placeholder)\n",
		m.runSeed.Text, m.rulesVersion, m.settings.Scarcity, m.settings.TextDensity, m.settings.Theme, m.settings.Language)
}

// --- Added implementations to fix missing methods ---

// renderTimeline displays the accumulated scene+outcome timeline with simple scrolling.
func (m *model) renderTimeline() string {
	title := "TIMELINE (PgUp/PgDn, Up/Down, Home/End, Esc back)"
	lines := strings.Split(m.timeline, "\n")
	w := m.width
	if w <= 0 {
		w = 100
	}
	h := m.height
	if h <= 0 {
		h = 30
	}
	avail := h - 2
	if avail < 1 {
		avail = len(lines)
	}
	start := m.timelineScroll
	if start < 0 {
		start = 0
	}
	maxStart := len(lines) - avail
	if maxStart < 0 {
		maxStart = 0
	}
	if start > maxStart {
		start = maxStart
	}
	m.timelineScroll = start
	view := lines
	if len(lines) > avail {
		end := start + avail
		if end > len(lines) {
			end = len(lines)
		}
		view = lines[start:end]
	}
	return title + "\n" + strings.Join(view, "\n")
}

func (m *model) renderProfileSelect() string {
	width := m.width
	if width < 68 {
		width = 68
	}
	height := m.height
	if height < 16 {
		height = 16
	}

	var lines []string
	lines = append(lines, "PROFILE SELECT")
	lines = append(lines, "")
	if len(m.profiles) == 0 {
		lines = append(lines, "(no profiles found)")
	} else {
		for i, p := range m.profiles {
			cursor := "  "
			if i == m.profileIndex {
				cursor = "> "
			}
			lines = append(lines, cursor+p.Name)
		}
	}
	if m.profileEditing {
		lines = append(lines, "")
		lines = append(lines, "New profile: "+m.styles.accent.Render(m.profileInput))
		lines = append(lines, m.styles.muted.Render("Enter to create, Esc to cancel."))
	} else {
		lines = append(lines, "")
		lines = append(lines, m.styles.muted.Render("Up/Down to choose, Enter to confirm, N to create."))
	}
	if msg := strings.TrimSpace(m.profileMessage); msg != "" {
		lines = append(lines, "")
		lines = append(lines, m.styles.muted.Render(msg))
	}
	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	box := m.styles.menuBox.Width(60).Render(body)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// cycleLanguage is a placeholder; persist a stable value or simple cycle.
func (m *model) cycleLanguage() {
	if m.runID == uuid.Nil {
		return
	}
	next := "en"
	// Placeholder: if more languages are supported in the future, cycle through them.
	if m.settings.Language != "en" {
		next = "en"
	}
	sr := store.NewSettingsRepo(m.db)
	_ = sr.Upsert(m.ctx, m.runID, m.settings.Scarcity, m.settings.TextDensity, next, m.settings.Narrator, m.settings.Difficulty, m.settings.Theme)
	m.refreshSettings()
}

func (m *model) cycleTheme() {
	if m.runID == uuid.Nil {
		m.preRunTheme = nextThemeName(m.preRunTheme, 1)
		m.applyTheme(m.preRunTheme)
		return
	}
	next := nextThemeName(m.settings.Theme, 1)
	sr := store.NewSettingsRepo(m.db)
	_ = sr.Upsert(m.ctx, m.runID, m.settings.Scarcity, m.settings.TextDensity, m.settings.Language, m.settings.Narrator, m.settings.Difficulty, next)
	m.refreshSettings()
	m.forceRegenerateChoices()
}
