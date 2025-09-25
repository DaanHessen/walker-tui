package store

import (
	"context"
	"database/sql"
	"encoding/json"
	errs "errors"
	"fmt"
	"strings"
	"time"

	"github.com/DaanHessen/walker-tui/internal/engine"
	"github.com/DaanHessen/walker-tui/internal/util"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ErrNoChange = errs.New("no change")

// DB wraps gorm.DB for repositories and exposes Close.
type DB struct {
	gorm *gorm.DB
	sql  *sql.DB
}

func (d *DB) Close() error   { return d.sql.Close() }
func (d *DB) Gorm() *gorm.DB { return d.gorm }

// Open connects to DB per config.
func Open(ctx context.Context, cfg util.Config) (*DB, error) {
	var (
		gdb *gorm.DB
		err error
	)
	if cfg.DSN == "" {
		return nil, fmt.Errorf("missing DSN")
	}
	// Postgres-only
	gdb, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sdb, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sdb.SetConnMaxLifetime(30 * time.Minute)
	sdb.SetMaxOpenConns(10)
	sdb.SetMaxIdleConns(5)
	if err := sdb.PingContext(ctx); err != nil {
		return nil, err
	}
	return &DB{gorm: gdb, sql: sdb}, nil
}

// Run model (DB layer minimal)
type Run struct {
	ID           uuid.UUID
	OriginSite   string
	CurrentDay   int
	SeedText     string
	RulesVersion string
	ProfileID    uuid.UUID
	LastPlayedAt time.Time
}

type Profile struct {
	ID         uuid.UUID
	Name       string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

type SurvivorRecord struct {
	ID              uuid.UUID
	RunID           uuid.UUID
	Name            string
	Age             int
	Background      string
	Region          string
	LocationType    string
	GroupType       string
	GroupSize       int
	Traits          []string
	SkillsJSON      json.RawMessage
	StatsJSON       json.RawMessage
	BodyTemp        string
	Conditions      []string
	MetersJSON      json.RawMessage
	InventoryJSON   json.RawMessage
	EnvironmentJSON json.RawMessage
	Alive           bool
}

// RunRepo basic operations.
type RunRepo struct{ db *DB }

func NewRunRepo(db *DB) *RunRepo { return &RunRepo{db: db} }

type ProfileRepo struct{ db *DB }

func NewProfileRepo(db *DB) *ProfileRepo { return &ProfileRepo{db: db} }

func (pr *ProfileRepo) List(ctx context.Context) ([]Profile, error) {
	rows, err := pr.db.gorm.WithContext(ctx).Raw(`SELECT id, name, created_at, last_used_at FROM profiles ORDER BY last_used_at DESC, created_at DESC`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var profiles []Profile
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.LastUsedAt); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func (pr *ProfileRepo) Create(ctx context.Context, name string) (Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Profile{}, errs.New("profile name required")
	}
	var p Profile
	row := pr.db.gorm.WithContext(ctx).Raw(`INSERT INTO profiles(id, name) VALUES(uuid_generate_v4(), ?) RETURNING id, name, created_at, last_used_at`, name).Row()
	if err := row.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.LastUsedAt); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func (pr *ProfileRepo) Touch(ctx context.Context, id uuid.UUID) error {
	return pr.db.gorm.WithContext(ctx).Exec(`UPDATE profiles SET last_used_at = now() WHERE id = ?`, id).Error
}

// CreateWithSeed inserts a run with canonical seed text for the provided profile.
func (r *RunRepo) CreateWithSeed(ctx context.Context, profileID uuid.UUID, origin, seedText, rulesVersion string) (Run, error) {
	if profileID == uuid.Nil {
		return Run{}, errs.New("profile id required")
	}
	id := uuid.New()
	if err := r.db.gorm.Exec(`INSERT INTO runs(id, profile_id, origin_site, seed, rules_version, last_played_at) VALUES(?,?,?,?,?,now())`, id, profileID, origin, seedText, rulesVersion).Error; err != nil {
		return Run{}, err
	}
	return Run{ID: id, OriginSite: origin, CurrentDay: 0, SeedText: seedText, RulesVersion: rulesVersion, ProfileID: profileID, LastPlayedAt: time.Now()}, nil
}

// Legacy Create retained for backwards compatibility.
func (r *RunRepo) Create(ctx context.Context, origin string, seed int64) (Run, error) {
	return Run{}, errs.New("profile id required")
}

// SurvivorRepo minimal creation.
type SurvivorRepo struct{ db *DB }

func NewSurvivorRepo(db *DB) *SurvivorRepo { return &SurvivorRepo{db: db} }

func (s *SurvivorRepo) Create(ctx context.Context, runID uuid.UUID, sv engine.Survivor) (uuid.UUID, error) {
	id := uuid.New()
	skills, _ := json.Marshal(sv.Skills)
	stats, _ := json.Marshal(sv.Stats)
	meters, _ := json.Marshal(sv.Meters)
	inv, _ := json.Marshal(sv.Inventory)
	env, _ := json.Marshal(sv.Environment)
	err := s.db.gorm.Exec(`INSERT INTO survivors(
		id, run_id, name, age, background, region, location_type, group_type, group_size, traits, skills, stats, body_temp, conditions, meters, inventory, environment, alive
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, runID, sv.Name, sv.Age, sv.Background, sv.Region, sv.Location, sv.Group, sv.GroupSize, pq.Array(pqStringArray(sv.Traits)), skills, stats, sv.BodyTemp, pq.Array(pqStringArray(sv.Conditions)), meters, inv, env, sv.Alive,
	).Error
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

type Scene struct {
	ID         uuid.UUID
	RunID      uuid.UUID
	SurvivorID uuid.UUID
	WorldDay   int
	Phase      string
	LAD        int
	Markdown   string
}

type Choice struct {
	ID       uuid.UUID
	SceneID  uuid.UUID
	Index    int
	Label    string
	CostJSON json.RawMessage
	Risk     string
}

type SceneRepo struct{ db *DB }

func NewSceneRepo(db *DB) *SceneRepo { return &SceneRepo{db: db} }

type ChoiceRepo struct{ db *DB }

func NewChoiceRepo(db *DB) *ChoiceRepo { return &ChoiceRepo{db: db} }

type Update struct {
	ID            uuid.UUID
	SceneID       uuid.UUID
	Deltas        json.RawMessage
	NewConditions []string
}

type Outcome struct {
	ID       uuid.UUID
	SceneID  uuid.UUID
	Markdown string
}

type ArchiveCard struct {
	ID               uuid.UUID
	RunID            uuid.UUID
	SurvivorID       uuid.UUID
	WorldDay         int
	Region           string
	CauseOfDeath     string
	Skills           []string
	NotableDecisions []string
	Allies           []string
	FinalInventory   json.RawMessage
	Markdown         string
}

type MasterLog struct {
	ID             uuid.UUID
	RunID          uuid.UUID
	SurvivorID     uuid.UUID
	ChoicesSummary json.RawMessage
	NarrativeRecap string
}

type UpdateRepo struct{ db *DB }

func NewUpdateRepo(db *DB) *UpdateRepo { return &UpdateRepo{db: db} }

type OutcomeRepo struct{ db *DB }

func NewOutcomeRepo(db *DB) *OutcomeRepo { return &OutcomeRepo{db: db} }

type ArchiveRepo struct{ db *DB }

func NewArchiveRepo(db *DB) *ArchiveRepo { return &ArchiveRepo{db: db} }

type LogRepo struct{ db *DB }

func NewLogRepo(db *DB) *LogRepo { return &LogRepo{db: db} }

type EventRepo struct{ db *DB }

func NewEventRepo(db *DB) *EventRepo { return &EventRepo{db: db} }

type NarrationCacheRepo struct{ db *DB }

func NewNarrationCacheRepo(db *DB) *NarrationCacheRepo { return &NarrationCacheRepo{db: db} }

type EventInstanceRecord struct {
	RunID              uuid.UUID
	SurvivorID         uuid.UUID
	EventID            string
	WorldDay           int
	SceneIdx           int
	CooldownUntilScene int
	OnceFired          bool
}

func (er *EventRepo) LoadHistory(ctx context.Context, tx *gorm.DB, runID uuid.UUID) (engine.EventHistory, error) {
	hist := engine.EventHistory{
		Events: make(map[string]engine.EventState),
	}
	if runID == uuid.Nil {
		return hist, nil
	}
	db := er.db.gorm.WithContext(ctx)
	if tx != nil {
		db = tx.WithContext(ctx)
	}
	rows, err := db.Raw(`SELECT event_id, scene_idx, cooldown_until_scene, once_fired FROM event_instances WHERE run_id = ? ORDER BY scene_idx DESC, created_at DESC`, runID).Rows()
	if err != nil {
		return hist, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			eventID   string
			sceneIdx  int
			cooldown  int
			onceFired bool
		)
		if err := rows.Scan(&eventID, &sceneIdx, &cooldown, &onceFired); err != nil {
			return hist, err
		}
		if _, ok := hist.Events[eventID]; !ok {
			hist.Events[eventID] = engine.EventState{
				LastSceneIdx:       sceneIdx,
				CooldownUntilScene: cooldown,
				OnceFired:          onceFired,
			}
		}
		hist.Recent = append(hist.Recent, eventID)
	}
	return hist, nil
}

func (er *EventRepo) Insert(ctx context.Context, tx *gorm.DB, rec EventInstanceRecord) error {
	db := er.db.gorm.WithContext(ctx)
	if tx != nil {
		db = tx.WithContext(ctx)
	}
	return db.Exec(`INSERT INTO event_instances(run_id, survivor_id, event_id, world_day, scene_idx, cooldown_until_scene, arc_id, arc_step, once_fired) VALUES (?,?,?,?,?,?,?,?,?)`,
		rec.RunID, rec.SurvivorID, rec.EventID, rec.WorldDay, rec.SceneIdx, rec.CooldownUntilScene, nil, 0, rec.OnceFired).Error
}

func (nr *NarrationCacheRepo) Get(ctx context.Context, tx *gorm.DB, runID uuid.UUID, kind string, hash []byte) (string, bool, error) {
	db := nr.db.gorm.WithContext(ctx)
	if tx != nil {
		db = tx.WithContext(ctx)
	}
	row := db.Raw(`SELECT text FROM narration_cache WHERE run_id = ? AND kind = ? AND state_hash = ?`, runID, kind, hash).Row()
	var text string
	if err := row.Scan(&text); err != nil {
		if errs.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return text, true, nil
}

func (nr *NarrationCacheRepo) Put(ctx context.Context, tx *gorm.DB, runID uuid.UUID, kind string, hash []byte, text string) error {
	db := nr.db.gorm.WithContext(ctx)
	if tx != nil {
		db = tx.WithContext(ctx)
	}
	return db.Exec(`INSERT INTO narration_cache(run_id, state_hash, kind, text) VALUES (?,?,?,?) ON CONFLICT DO NOTHING`, runID, hash, kind, text).Error
}

// WithTx executes fn within a database transaction.
func (d *DB) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return d.gorm.WithContext(ctx).Transaction(fn)
}

// Helper converts []T (string-like) to []string for driver.
func pqStringArray[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func (r *RunRepo) Get(ctx context.Context, id uuid.UUID) (Run, error) {
	row := r.db.gorm.WithContext(ctx).Raw(`SELECT id, origin_site, current_day, COALESCE(seed, ''), COALESCE(rules_version,'1.0.0'), profile_id, COALESCE(last_played_at, now()) FROM runs WHERE id = ?`, id).Row()
	var rr Run
	if err := row.Scan(&rr.ID, &rr.OriginSite, &rr.CurrentDay, &rr.SeedText, &rr.RulesVersion, &rr.ProfileID, &rr.LastPlayedAt); err != nil {
		return Run{}, err
	}
	return rr, nil
}

// GetLatestRun returns most recently played run for profile.
func (r *RunRepo) GetLatestRun(ctx context.Context, profileID uuid.UUID) (Run, error) {
	row := r.db.gorm.WithContext(ctx).Raw(`SELECT id, origin_site, current_day, COALESCE(seed, ''), COALESCE(rules_version,'1.0.0'), profile_id, COALESCE(last_played_at, now()) FROM runs WHERE profile_id = ? ORDER BY last_played_at DESC LIMIT 1`, profileID).Row()
	var rr Run
	if err := row.Scan(&rr.ID, &rr.OriginSite, &rr.CurrentDay, &rr.SeedText, &rr.RulesVersion, &rr.ProfileID, &rr.LastPlayedAt); err != nil {
		return Run{}, err
	}
	return rr, nil
}

// SurvivorRepo additions
func (s *SurvivorRepo) Get(ctx context.Context, id uuid.UUID) (engine.Survivor, error) {
	row := s.db.gorm.Raw(`SELECT name, age, background, region, location_type, group_type, group_size, traits, skills, stats, body_temp, conditions, meters, inventory, environment, alive FROM survivors WHERE id = ?`, id).Row()
	var (
		name, background, region, locationType, groupType, bodyTemp string
		age, groupSize                                              int
		traits, conditions                                          []byte
		skillsB, statsB, metersB, invB, envB                        []byte
		alive                                                       bool
	)
	if err := row.Scan(&name, &age, &background, &region, &locationType, &groupType, &groupSize, &traits, &skillsB, &statsB, &bodyTemp, &conditions, &metersB, &invB, &envB, &alive); err != nil {
		return engine.Survivor{}, err
	}
	// Minimal unmarshal for now (omitted for brevity) – return placeholder.
	return engine.Survivor{Name: name, Age: age, Background: background, Region: region, Location: engine.LocationType(locationType), Group: engine.GroupType(groupType), GroupSize: groupSize, Alive: alive}, nil
}

// GetAliveSurvivor returns latest alive survivor for run (simple max updated_at ordering).
func (s *SurvivorRepo) GetAliveSurvivor(ctx context.Context, runID uuid.UUID) (engine.Survivor, uuid.UUID, error) {
	row := s.db.gorm.WithContext(ctx).Raw(`SELECT id, name, age, background, region, location_type, group_type, group_size, traits, skills, stats, body_temp, conditions, meters, inventory, environment, alive FROM survivors WHERE run_id = ? AND alive = TRUE ORDER BY updated_at DESC LIMIT 1`, runID).Row()
	var (
		id                                                          uuid.UUID
		name, background, region, locationType, groupType, bodyTemp string
		age, groupSize                                              int
		traitsArr, condsArr                                         []string
		skillsB, statsB, metersB, invB, envB                        []byte
		alive                                                       bool
	)
	if err := row.Scan(&id, &name, &age, &background, &region, &locationType, &groupType, &groupSize, pq.Array(&traitsArr), &skillsB, &statsB, &bodyTemp, pq.Array(&condsArr), &metersB, &invB, &envB, &alive); err != nil {
		return engine.Survivor{}, uuid.Nil, err
	}
	var skills map[engine.Skill]int
	_ = json.Unmarshal(skillsB, &skills)
	var stats engine.Stats
	_ = json.Unmarshal(statsB, &stats)
	var meters map[engine.Meter]int
	_ = json.Unmarshal(metersB, &meters)
	var inv engine.Inventory
	_ = json.Unmarshal(invB, &inv)
	var env engine.Environment
	_ = json.Unmarshal(envB, &env)
	traits := make([]engine.Trait, len(traitsArr))
	for i, t := range traitsArr {
		traits[i] = engine.Trait(t)
	}
	conds := make([]engine.Condition, len(condsArr))
	for i, c := range condsArr {
		conds[i] = engine.Condition(c)
	}
	surv := engine.Survivor{Name: name, Age: age, Background: background, Region: region, Location: engine.LocationType(locationType), Group: engine.GroupType(groupType), GroupSize: groupSize, Traits: traits, Skills: skills, Stats: stats, BodyTemp: engine.TempBand(bodyTemp), Conditions: conds, Meters: meters, Inventory: inv, Environment: env, Alive: alive}
	return surv, id, nil
}

// SceneRepo persistence
func (sr *SceneRepo) Insert(ctx context.Context, tx *gorm.DB, runID, survivorID uuid.UUID, worldDay int, phase string, lad int, md string) (uuid.UUID, error) {
	id := uuid.New()
	if err := tx.Exec(`INSERT INTO scenes(id, run_id, survivor_id, world_day, phase, lad, scene_md) VALUES (?,?,?,?,?,?,?)`, id, runID, survivorID, worldDay, phase, lad, md).Error; err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ChoiceRepo persistence
func (cr *ChoiceRepo) BulkInsert(ctx context.Context, tx *gorm.DB, sceneID uuid.UUID, choices []engine.Choice) error {
	for i, c := range choices {
		costB, _ := json.Marshal(c.Cost)
		if err := tx.Exec(`INSERT INTO choices(id, scene_id, idx, label, cost, risk) VALUES (?,?,?,?,?,?)`, uuid.New(), sceneID, i, c.Label, costB, c.Risk).Error; err != nil {
			return err
		}
	}
	return nil
}

// UpdateRepo persistence
func (ur *UpdateRepo) Insert(ctx context.Context, tx *gorm.DB, sceneID uuid.UUID, deltas engine.Stats, added, removed []engine.Condition) (uuid.UUID, error) {
	id := uuid.New()
	dB, _ := json.Marshal(deltas)
	addArr := pqStringArray(added)
	remArr := pqStringArray(removed)
	if err := tx.Exec(`INSERT INTO updates(id, scene_id, deltas, conditions_added, conditions_removed) VALUES (?,?,?,?,?)`, id, sceneID, dB, pq.Array(addArr), pq.Array(remArr)).Error; err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// OutcomeRepo persistence
func (or *OutcomeRepo) Insert(ctx context.Context, tx *gorm.DB, sceneID uuid.UUID, md string) (uuid.UUID, error) {
	id := uuid.New()
	if err := tx.Exec(`INSERT INTO outcomes(id, scene_id, outcome_md) VALUES (?,?,?)`, id, sceneID, md).Error; err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ArchiveRepo persistence
func (ar *ArchiveRepo) Insert(ctx context.Context, tx *gorm.DB, runID, survivorID uuid.UUID, worldDay int, region, cause string, keySkills []string, notable []string, allies []string, finalInv any, card string) (uuid.UUID, error) {
	id := uuid.New()
	invB, _ := json.Marshal(finalInv)
	exec := tx
	if exec == nil {
		exec = ar.db.gorm
	}
	if err := exec.Exec(`INSERT INTO archive_cards(id, run_id, survivor_id, world_day, region, cause_of_death, key_skills, notable_decisions, allies, final_inventory, card_md) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, id, runID, survivorID, worldDay, region, cause, pq.Array(keySkills), pq.Array(notable), pq.Array(allies), invB, card).Error; err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// helper for []string passthrough (without generic constraints reuse simplicity)
func pqStringArrayStr(in []string) []string { return append([]string{}, in...) }

// List returns archive cards for a run (most recent first by world_day then id) limited by provided cap.
func (ar *ArchiveRepo) List(ctx context.Context, runID uuid.UUID, limit int) ([]ArchiveCard, error) {
	rows, err := ar.db.gorm.WithContext(ctx).Raw(`SELECT id, run_id, survivor_id, world_day, region, cause_of_death, key_skills, notable_decisions, allies, final_inventory, card_md FROM archive_cards WHERE run_id = ? ORDER BY world_day DESC, id DESC LIMIT ?`, runID, limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ArchiveCard
	for rows.Next() {
		var ac ArchiveCard
		if err := rows.Scan(&ac.ID, &ac.RunID, &ac.SurvivorID, &ac.WorldDay, &ac.Region, &ac.CauseOfDeath, &ac.Skills, &ac.NotableDecisions, &ac.Allies, &ac.FinalInventory, &ac.Markdown); err != nil {
			return nil, err
		}
		res = append(res, ac)
	}
	return res, nil
}

// SettingsRepo
type SettingsRepo struct{ db *DB }

func NewSettingsRepo(db *DB) *SettingsRepo { return &SettingsRepo{db: db} }
func (sr *SettingsRepo) Upsert(ctx context.Context, runID uuid.UUID, scarcity bool, density, language, narrator, difficulty, theme string) error {
	return sr.db.gorm.WithContext(ctx).Exec(`INSERT INTO settings(run_id, scarcity, text_density, language, narrator, difficulty, theme) VALUES (?,?,?,?,?,?,?)
	ON CONFLICT (run_id) DO UPDATE SET scarcity=EXCLUDED.scarcity, text_density=EXCLUDED.text_density, language=EXCLUDED.language, narrator=EXCLUDED.narrator, difficulty=EXCLUDED.difficulty, theme=EXCLUDED.theme`, runID, scarcity, density, language, narrator, difficulty, theme).Error
}

// Backwards-compatible wrapper for existing call sites without difficulty/theme (defaults to standard + catppuccin).
func (sr *SettingsRepo) UpsertLegacy(ctx context.Context, runID uuid.UUID, scarcity bool, density, language, narrator string) error {
	return sr.Upsert(ctx, runID, scarcity, density, language, narrator, "standard", "catppuccin")
}

func (sr *SettingsRepo) ToggleScarcity(ctx context.Context, runID uuid.UUID) error {
	return sr.db.gorm.WithContext(ctx).Exec(`UPDATE settings SET scarcity = NOT scarcity WHERE run_id = ?`, runID).Error
}
func (sr *SettingsRepo) CycleDensity(ctx context.Context, runID uuid.UUID) error {
	return sr.db.gorm.WithContext(ctx).Exec(`UPDATE settings SET text_density = CASE text_density WHEN 'concise' THEN 'standard' WHEN 'standard' THEN 'rich' ELSE 'concise' END WHERE run_id = ?`, runID).Error
}
func (sr *SettingsRepo) ToggleNarrator(ctx context.Context, runID uuid.UUID) error {
	return sr.db.gorm.WithContext(ctx).Exec(`UPDATE settings SET narrator = CASE narrator WHEN 'off' THEN 'auto' ELSE 'off' END WHERE run_id = ?`, runID).Error
}
func (sr *SettingsRepo) CycleDifficulty(ctx context.Context, runID uuid.UUID) error {
	return sr.db.gorm.WithContext(ctx).Exec(`UPDATE settings SET difficulty = CASE difficulty WHEN 'easy' THEN 'standard' WHEN 'standard' THEN 'hard' ELSE 'easy' END WHERE run_id = ?`, runID).Error
}

// Get retrieves current settings for run.

func (sr *SettingsRepo) Get(ctx context.Context, runID uuid.UUID) (Settings, error) {
	row := sr.db.gorm.WithContext(ctx).Raw(`SELECT run_id, scarcity, text_density, language, narrator, COALESCE(difficulty,'standard'), COALESCE(theme,'catppuccin') FROM settings WHERE run_id = ?`, runID).Row()
	var s Settings
	if err := row.Scan(&s.RunID, &s.Scarcity, &s.TextDensity, &s.Language, &s.Narrator, &s.Difficulty, &s.Theme); err != nil {
		return Settings{}, err
	}
	return s, nil
}

type Tip struct {
	ID        uuid.UUID
	Text      string
	CreatedAt time.Time
}

type TipRepo struct{ db *DB }

func NewTipRepo(db *DB) *TipRepo { return &TipRepo{db: db} }

func (tr *TipRepo) All(ctx context.Context) ([]Tip, error) {
	rows, err := tr.db.gorm.WithContext(ctx).Raw(`SELECT id, text, created_at FROM tips ORDER BY created_at`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tips []Tip
	for rows.Next() {
		var tip Tip
		if err := rows.Scan(&tip.ID, &tip.Text, &tip.CreatedAt); err != nil {
			return nil, err
		}
		tips = append(tips, tip)
	}
	return tips, nil
}

// SurvivorRepo Update method for transactional survivor state persistence.
func (s *SurvivorRepo) Update(ctx context.Context, tx *gorm.DB, id uuid.UUID, sv engine.Survivor) error {
	skills, _ := json.Marshal(sv.Skills)
	stats, _ := json.Marshal(sv.Stats)
	meters, _ := json.Marshal(sv.Meters)
	inv, _ := json.Marshal(sv.Inventory)
	env, _ := json.Marshal(sv.Environment)
	conds := pqStringArray(sv.Conditions)
	exec := s.db.gorm.WithContext(ctx)
	if tx != nil {
		exec = tx.WithContext(ctx)
	}
	return exec.Exec(`UPDATE survivors SET skills = ?, stats = ?, body_temp = ?, conditions = ?, meters = ?, inventory = ?, environment = ?, alive = ? WHERE id = ?`,
		skills, stats, sv.BodyTemp, pq.Array(conds), meters, inv, env, sv.Alive, id).Error
}

// RunRepo day update
func (r *RunRepo) UpdateDay(ctx context.Context, tx *gorm.DB, id uuid.UUID, day int) error {
	exec := r.db.gorm.WithContext(ctx)
	if tx != nil {
		exec = tx.WithContext(ctx)
	}
	return exec.Exec(`UPDATE runs SET current_day = ?, last_played_at = now() WHERE id = ?`, day, id).Error
}

// LogRepo insert master log
func (lr *LogRepo) Insert(ctx context.Context, tx *gorm.DB, runID, survivorID uuid.UUID, summary any, recap string) (uuid.UUID, error) {
	id := uuid.New()
	b, _ := json.Marshal(summary)
	if err := tx.Exec(`INSERT INTO master_logs(id, run_id, survivor_id, choices_summary, narrative_recap) VALUES (?,?,?,?,?)`, id, runID, survivorID, b, recap).Error; err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ListRecent returns up to limit recent master log entries (ordered by id as placeholder ordering)
func (lr *LogRepo) ListRecent(ctx context.Context, runID uuid.UUID, limit int) ([]MasterLog, error) {
	rows, err := lr.db.gorm.WithContext(ctx).Raw(`SELECT id, run_id, survivor_id, choices_summary, narrative_recap FROM master_logs WHERE run_id = ? ORDER BY id DESC LIMIT ?`, runID, limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MasterLog
	for rows.Next() {
		var ml MasterLog
		if err := rows.Scan(&ml.ID, &ml.RunID, &ml.SurvivorID, &ml.ChoicesSummary, &ml.NarrativeRecap); err != nil {
			return nil, err
		}
		out = append(out, ml)
	}
	return out, nil
}

type Settings struct {
	RunID       uuid.UUID
	Scarcity    bool
	TextDensity string
	Language    string
	Narrator    string
	Difficulty  string
	Theme       string
}

type SceneWithOutcome struct {
	WorldDay  int
	SceneMD   string
	OutcomeMD string
}

// ScenesWithOutcomes returns scenes joined with outcomes chronologically for a run.
func (sr *SceneRepo) ScenesWithOutcomes(ctx context.Context, runID uuid.UUID) ([]SceneWithOutcome, error) {
	rows, err := sr.db.gorm.WithContext(ctx).Raw(`SELECT s.world_day, s.scene_md, COALESCE(o.outcome_md,'') FROM scenes s LEFT JOIN outcomes o ON o.scene_id = s.id WHERE s.run_id = ? ORDER BY s.world_day, s.id`, runID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []SceneWithOutcome
	for rows.Next() {
		var swo SceneWithOutcome
		if err := rows.Scan(&swo.WorldDay, &swo.SceneMD, &swo.OutcomeMD); err != nil {
			return nil, err
		}
		res = append(res, swo)
	}
	return res, nil
}
