package engine

import (
	"math/rand"
	"time"
)

// World holds run-wide data.
type World struct {
	OriginSite string
	Seed       int64
	CurrentDay int // advances globally; survivors spawn into this
}

// Survivor represents an in-game character.
type Survivor struct {
	Name        string
	Age         int
	Background  string
	Region      string
	Location    LocationType
	Group       GroupType
	GroupSize   int
	Traits      []Trait
	Skills      map[Skill]int // 0-5 inclusive
	Stats       Stats
	BodyTemp    TempBand
	Conditions  []Condition
	Meters      map[Meter]int // 0-100 internal scaling for now
	Inventory   Inventory
	Environment Environment
	Alive       bool
}

type Stats struct {
	Health  int
	Hunger  int
	Thirst  int
	Fatigue int
	Morale  int
}

type Inventory struct {
	Weapons      []string
	Ammo         map[string]int
	FoodDays     float64
	WaterLiters  float64
	Medical      []string
	Tools        []string
	Special      []string
	Memento      string
}

type Environment struct {
	WorldDay    int
	TimeOfDay   string
	Season      Season
	Weather     Weather
	TempBand    TempBand
	Region      string
	Location    LocationType
	LAD         int // Local Arrival Day precomputed for gating
	Infected    bool // whether infected are now present locally (day >= LAD)
}

// RNG returns a deterministic rand.Rand for a seed.
func RNG(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

// ComputeLAD calculates the Local Arrival Day for a region vs origin and modifiers.
// Placeholder simplistic formula: distance tier only.
func ComputeLAD(distanceKM float64, tierModifiers int) int {
	var base int
	switch {
	case distanceKM <= 100:
		base = 0 // Tier A
	case distanceKM <= 800: // heuristic national / adjacent heavy transit
		base = 2 // Tier B midpoint (1-3)
	case distanceKM <= 3000: // same continent
		base = 6 // Tier C midpoint (3-10)
	default:
		base = 14 // Tier D midpoint (7-21)
	}
	lad := base + tierModifiers
	if lad < 0 { lad = 0 }
	return lad
}

// Clamp stat into 0-100.
func Clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// NewFirstSurvivor generates the initial survivor per first-run rule.
func NewFirstSurvivor(r *rand.Rand, worldDay int, originRegion string) Survivor {
	traits := []Trait{AllTraits[r.Intn(len(AllTraits))]}
	// Ensure at least 2 traits distinct
	for len(traits) < 2 {
		t := AllTraits[r.Intn(len(AllTraits))]
		dup := false
		for _, ex := range traits { if ex == t { dup = true; break } }
		if !dup { traits = append(traits, t) }
	}
	skills := make(map[Skill]int)
	for _, s := range AllSkills { skills[s] = 0 }
	return Survivor{
		Name:       randomName(r),
		Age:        18 + r.Intn(38),
		Background: "civilian",
		Region:     originRegion,
		Location:   LocationSuburb,
		Group:      GroupSolo,
		GroupSize:  1,
		Traits:     traits,
		Skills:     skills,
		Stats:      Stats{Health: 100, Hunger: 30, Thirst: 30, Fatigue: 10, Morale: 60},
		BodyTemp:   TempMild,
		Conditions: nil,
		Meters:     map[Meter]int{MeterNoise: 0, MeterVisibility: 0, MeterScent: 0},
		Inventory:  Inventory{Weapons: nil, Ammo: map[string]int{}, FoodDays: 0.5, WaterLiters: 1.0, Medical: []string{"bandage"}, Tools: []string{"pocket knife"}},
		Environment: Environment{WorldDay: worldDay, TimeOfDay: "morning", Season: SeasonSpring, Weather: WeatherClear, TempBand: TempMild, Region: originRegion, Location: LocationSuburb, LAD: 0, Infected: worldDay >= 0},
		Alive: true,
	}
}

func randomName(r *rand.Rand) string {
	names := []string{"Alex", "Jordan", "Taylor", "Riley", "Morgan", "Casey", "Jamie", "Avery"}
	return names[r.Intn(len(names))]
}

// AdvanceDay increments global day.
func (w *World) AdvanceDay() { w.CurrentDay++ }

// UpdateStats applies drains and clamps.
func (s *Survivor) UpdateStats(delta Stats) {
	s.Stats.Health = Clamp(s.Stats.Health + delta.Health)
	s.Stats.Hunger = Clamp(s.Stats.Hunger + delta.Hunger)
	s.Stats.Thirst = Clamp(s.Stats.Thirst + delta.Thirst)
	s.Stats.Fatigue = Clamp(s.Stats.Fatigue + delta.Fatigue)
	s.Stats.Morale = Clamp(s.Stats.Morale + delta.Morale)
}

// Tick increases hunger/thirst/fatigue baseline.
func (s *Survivor) Tick() {
	s.UpdateStats(Stats{Hunger: 5, Thirst: 7, Fatigue: 3})
	if s.Stats.Thirst >= 90 || s.Stats.Hunger >= 95 {
		s.UpdateStats(Stats{Health: -5, Morale: -3})
	}
}

// IsDead returns if survivor is dead.
func (s *Survivor) IsDead() bool { return s.Stats.Health <= 0 }

// Simple death check logic placeholder.
func (s *Survivor) EvaluateDeath() {
	if s.IsDead() { s.Alive = false }
}

// World initialization helper.
func NewWorld(seed int64) *World {
	return &World{OriginSite: pickOrigin(seed), Seed: seed, CurrentDay: 0}
}

func pickOrigin(seed int64) string {
	sites := []string{
		"USAMRIID/Fort Detrick (USA)",
		"Galveston National Lab (USA)",
		"Porton Down (UK)",
		"Vector Institute (Russia)",
		"Riems Island Lab (Germany)",
		"Wuhan Institute of Virology (China)",
	}
	r := rand.New(rand.NewSource(seed))
	return sites[r.Intn(len(sites))]
}

// Example placeholder scene state marshaling.
func (s Survivor) NarrativeState() map[string]any {
	return map[string]any{
		"name": s.Name,
		"age": s.Age,
		"traits": s.Traits,
		"skills": s.Skills,
		"stats": s.Stats,
		"location": s.Location,
		"region": s.Region,
		"world_day": s.Environment.WorldDay,
		"lad": s.Environment.LAD,
		"infected_present": s.Environment.Infected,
		"conditions": s.Conditions,
	}
}

// SeedTime returns deterministic time for run day.
func SeedTime(seed int64, day int) time.Time { return time.Unix(seed, 0).Add(time.Duration(day) * 24 * time.Hour) }

// SyncEnvironmentDay updates the environment's world day and infection presence.
func (s *Survivor) SyncEnvironmentDay(day int) {
	s.Environment.WorldDay = day
	s.updateInfectionPresence()
}

func (s *Survivor) updateInfectionPresence() {
	s.Environment.Infected = s.Environment.WorldDay >= s.Environment.LAD
}
