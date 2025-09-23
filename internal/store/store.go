package store

import (
	"context"
	"database/sql"
	"encoding/json"
	errs "errors"
	"fmt"
	"time"

	"github.com/DaanHessen/walker-tui/internal/engine"
	"github.com/DaanHessen/walker-tui/internal/util"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ErrNoChange = errs.New("no change")

// DB wraps gorm.DB for repositories and exposes Close.
type DB struct {
	gorm *gorm.DB
	sql  *sql.DB
}

func (d *DB) Close() error { return d.sql.Close() }
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
	ID         uuid.UUID
	OriginSite string
	Seed       int64
	CurrentDay int
}

type SurvivorRecord struct {
	ID        uuid.UUID
	RunID     uuid.UUID
	Name      string
	Age       int
	Background string
	Region    string
	LocationType string
	GroupType string
	GroupSize int
	Traits    []string
	SkillsJSON  json.RawMessage
	StatsJSON   json.RawMessage
	BodyTemp  string
	Conditions []string
	MetersJSON json.RawMessage
	InventoryJSON json.RawMessage
	EnvironmentJSON json.RawMessage
	Alive bool
}

// RunRepo basic operations.
type RunRepo struct { db *DB }
func NewRunRepo(db *DB) *RunRepo { return &RunRepo{db: db} }
func (r *RunRepo) Create(ctx context.Context, origin string, seed int64) (Run, error) {
	id := uuid.New()
	if err := r.db.gorm.Exec(`INSERT INTO runs(id, origin_site, seed) VALUES(?,?,?)`, id, origin, seed).Error; err != nil { return Run{}, err }
	return Run{ID: id, OriginSite: origin, Seed: seed, CurrentDay: 0}, nil
}

// SurvivorRepo minimal creation.
type SurvivorRepo struct { db *DB }
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
		id, runID, sv.Name, sv.Age, sv.Background, sv.Region, sv.Location, sv.Group, sv.GroupSize, pqStringArray(sv.Traits), skills, stats, sv.BodyTemp, pqStringArray(sv.Conditions), meters, inv, env, sv.Alive,
	).Error
	if err != nil { return uuid.Nil, err }
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
	ID      uuid.UUID
	SceneID uuid.UUID
	Index   int
	Label   string
	CostJSON json.RawMessage
	Risk    string
}

type SceneRepo struct { db *DB }
func NewSceneRepo(db *DB) *SceneRepo { return &SceneRepo{db: db} }

type ChoiceRepo struct { db *DB }
func NewChoiceRepo(db *DB) *ChoiceRepo { return &ChoiceRepo{db: db} }

type Update struct {
	ID uuid.UUID
	SceneID uuid.UUID
	Deltas json.RawMessage
	NewConditions []string
}

type Outcome struct {
	ID uuid.UUID
	SceneID uuid.UUID
	Markdown string
}

type ArchiveCard struct {
	ID uuid.UUID
	RunID uuid.UUID
	SurvivorID uuid.UUID
	WorldDay int
	Region string
	CauseOfDeath string
	KeySkills []string
	NotableDecisions []string
	Allies []string
	FinalInventory json.RawMessage
	Markdown string
}

type MasterLog struct {
	ID uuid.UUID
	RunID uuid.UUID
	SurvivorID uuid.UUID
	ChoicesSummary json.RawMessage
	NarrativeRecap string
}

type UpdateRepo struct { db *DB }
func NewUpdateRepo(db *DB) *UpdateRepo { return &UpdateRepo{db: db} }

type OutcomeRepo struct { db *DB }
func NewOutcomeRepo(db *DB) *OutcomeRepo { return &OutcomeRepo{db: db} }

type ArchiveRepo struct { db *DB }
func NewArchiveRepo(db *DB) *ArchiveRepo { return &ArchiveRepo{db: db} }

type LogRepo struct { db *DB }
func NewLogRepo(db *DB) *LogRepo { return &LogRepo{db: db} }

// WithTx executes fn within a database transaction.
func (d *DB) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return d.gorm.WithContext(ctx).Transaction(fn)
}

// Helper converts []T (string-like) to []string for driver.
func pqStringArray[T ~string](in []T) []string { out := make([]string, len(in)); for i, v := range in { out[i] = string(v) }; return out }

// RunRepo additions
func (r *RunRepo) Get(ctx context.Context, id uuid.UUID) (Run, error) {
	row := r.db.gorm.Raw(`SELECT id, origin_site, seed, current_day FROM runs WHERE id = ?`, id).Row()
	var rr Run
	if err := row.Scan(&rr.ID, &rr.OriginSite, &rr.Seed, &rr.CurrentDay); err != nil { return Run{}, err }
	return rr, nil
}

// SurvivorRepo additions
func (s *SurvivorRepo) Get(ctx context.Context, id uuid.UUID) (engine.Survivor, error) {
	row := s.db.gorm.Raw(`SELECT name, age, background, region, location_type, group_type, group_size, traits, skills, stats, body_temp, conditions, meters, inventory, environment, alive FROM survivors WHERE id = ?`, id).Row()
	var (
		name, background, region, locationType, groupType, bodyTemp string
		age, groupSize int
		traits, conditions []byte
		skillsB, statsB, metersB, invB, envB []byte
		alive bool
	)
	if err := row.Scan(&name, &age, &background, &region, &locationType, &groupType, &groupSize, &traits, &skillsB, &statsB, &bodyTemp, &conditions, &metersB, &invB, &envB, &alive); err != nil {
		return engine.Survivor{}, err
	}
	// Minimal unmarshal for now (omitted for brevity) – return placeholder.
	return engine.Survivor{Name: name, Age: age, Background: background, Region: region, Location: engine.LocationType(locationType), Group: engine.GroupType(groupType), GroupSize: groupSize, Alive: alive}, nil
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
func (ur *UpdateRepo) Insert(ctx context.Context, tx *gorm.DB, sceneID uuid.UUID, deltas engine.Stats, newConditions []engine.Condition) (uuid.UUID, error) {
	id := uuid.New()
	dB, _ := json.Marshal(deltas)
	conds := pqStringArray(newConditions)
	if err := tx.Exec(`INSERT INTO updates(id, scene_id, deltas, new_conditions) VALUES (?,?,?,?)`, id, sceneID, dB, conds).Error; err != nil { return uuid.Nil, err }
	return id, nil
}

// OutcomeRepo persistence
func (or *OutcomeRepo) Insert(ctx context.Context, tx *gorm.DB, sceneID uuid.UUID, md string) (uuid.UUID, error) {
	id := uuid.New()
	if err := tx.Exec(`INSERT INTO outcomes(id, scene_id, outcome_md) VALUES (?,?,?)`, id, sceneID, md).Error; err != nil { return uuid.Nil, err }
	return id, nil
}

// ArchiveRepo persistence
func (ar *ArchiveRepo) Insert(ctx context.Context, tx *gorm.DB, runID, survivorID uuid.UUID, worldDay int, region, cause string, finalInv any, card string) (uuid.UUID, error) {
	id := uuid.New()
	invB, _ := json.Marshal(finalInv)
	if err := tx.Exec(`INSERT INTO archive_cards(id, run_id, survivor_id, world_day, region, cause_of_death, key_skills, notable_decisions, allies, final_inventory, card_md) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, id, runID, survivorID, worldDay, region, cause, pqStringArray([]engine.Skill{}), pqStringArray([]engine.Trait{}), pqStringArray([]engine.Trait{}), invB, card).Error; err != nil { return uuid.Nil, err }
	return id, nil
}

// SettingsRepo
type SettingsRepo struct { db *DB }
func NewSettingsRepo(db *DB) *SettingsRepo { return &SettingsRepo{db: db} }
func (sr *SettingsRepo) Upsert(ctx context.Context, runID uuid.UUID, scarcity bool, density, language, narrator string) error {
	return sr.db.gorm.WithContext(ctx).Exec(`INSERT INTO settings(run_id, scarcity, text_density, language, narrator) VALUES (?,?,?,?,?)
	ON CONFLICT (run_id) DO UPDATE SET scarcity=EXCLUDED.scarcity, text_density=EXCLUDED.text_density, language=EXCLUDED.language, narrator=EXCLUDED.narrator`, runID, scarcity, density, language, narrator).Error
}
func (sr *SettingsRepo) ToggleScarcity(ctx context.Context, runID uuid.UUID) error {
	return sr.db.gorm.WithContext(ctx).Exec(`UPDATE settings SET scarcity = NOT scarcity WHERE run_id = ?`, runID).Error
}
func (sr *SettingsRepo) CycleDensity(ctx context.Context, runID uuid.UUID) error {
	return sr.db.gorm.WithContext(ctx).Exec(`UPDATE settings SET text_density = CASE text_density WHEN 'concise' THEN 'standard' WHEN 'standard' THEN 'rich' ELSE 'concise' END WHERE run_id = ?`, runID).Error
}

// Helper error wrap
func wrap(err error, msg string) error { if err == nil { return nil }; return errors.Wrap(err, msg) }
