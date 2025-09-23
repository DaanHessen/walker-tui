package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/DaanHessen/walker-tui/internal/engine"
	"github.com/DaanHessen/walker-tui/internal/util"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ErrNoChange = errors.New("no change")

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

// Helper converts []T (string-like) to []string for driver.
func pqStringArray[T ~string](in []T) []string { out := make([]string, len(in)); for i, v := range in { out[i] = string(v) }; return out }
