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
	Weapons     []string
	Ammo        map[string]int
	FoodDays    float64
	WaterLiters float64
	Medical     []string
	Tools       []string
	Special     []string
	Memento     string
}

type Environment struct {
	WorldDay  int
	TimeOfDay string
	Season    Season
	Weather   Weather
	TempBand  TempBand
	Region    string
	Location  LocationType
	LAD       int
	Infected  bool
	Timezone  string // IANA timezone identifier
}

// RNG returns a deterministic rand.Rand for a seed.
func RNG(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

// ComputeLAD calculates Local Arrival Day based on distance and simple modifiers.
// distanceKM bucketed into tiers; modifiers adjust within tier bounds.
// hub: -1 day (min 0); rural: +1 day; closures: +1; evac wave: jitter +/-1.
func ComputeLAD(distanceKM float64, hub bool, rural bool, closures bool, evac bool, seed int64) int {
	var baseMin, baseMax int
	switch {
	case distanceKM <= 100:
		baseMin, baseMax = 0, 0 // Tier A immediate
	case distanceKM <= 800:
		baseMin, baseMax = 1, 3 // Tier B
	case distanceKM <= 3000:
		baseMin, baseMax = 3, 10 // Tier C
	default:
		baseMin, baseMax = 7, 21 // Tier D
	}
	// midpoint start
	mid := (baseMin + baseMax) / 2
	if hub {
		mid -= 1
	}
	if rural {
		mid += 1
	}
	if closures {
		mid += 1
	}
	if mid < baseMin {
		mid = baseMin
	}
	if mid > baseMax {
		mid = baseMax
	}
	if evac {
		r := rand.New(rand.NewSource(seed + int64(mid)))
		mid += r.Intn(3) - 1 // -1..+1
		if mid < baseMin {
			mid = baseMin
		}
		if mid > baseMax {
			mid = baseMax
		}
	}
	return mid
}

// deriveInitialLAD produces a plausible Local Arrival Day for a survivor's starting location.
// This is a heuristic placeholder until a richer geospatial model is implemented.
// We approximate distance bands based on location type and apply random flags.
func deriveInitialLAD(r *rand.Rand, originSite string, loc LocationType, seed int64) int {
	// Approximate distance from outbreak origin (km) by location type distribution.
	var distance float64
	switch loc {
	case LocationCity:
		// Closer on average
		distance = 50 + r.Float64()*150 // 50-200
	case LocationSuburb:
		distance = 150 + r.Float64()*600 // 150-750
	case LocationRural:
		distance = 400 + r.Float64()*2600 // 400-3000
	default:
		distance = 500 + r.Float64()*2000
	}
	// Modifiers.
	hub := loc == LocationCity
	ruralFlag := loc == LocationRural
	closures := r.Float64() < 0.15 // 15% chance of transport closures slowing spread
	evac := r.Float64() < 0.20     // 20% chance evacuation wave introduces jitter
	lad := ComputeLAD(distance, hub, ruralFlag, closures, evac, seed+int64(r.Intn(9999)))
	if lad < 0 {
		lad = 0
	}
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

var surnames = []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Wilson", "Moore", "Taylor", "Anderson", "Thomas", "Jackson", "White", "Harris", "Martin", "Thompson", "Martinez", "Robinson"}

// profession templates with baseline stat ranges and starting inventory tweaks
type professionTemplate struct {
	Name         string
	HealthRange  [2]int
	HungerRange  [2]int
	ThirstRange  [2]int
	FatigueRange [2]int
	MoraleRange  [2]int
	InventoryFn  func(inv *Inventory)
}

var professionTemplates = []professionTemplate{
	{"nurse", [2]int{85, 100}, [2]int{20, 35}, [2]int{20, 35}, [2]int{5, 15}, [2]int{55, 70}, func(inv *Inventory) { inv.Medical = append(inv.Medical, "antiseptic") }},
	{"mechanic", [2]int{90, 105}, [2]int{25, 40}, [2]int{25, 40}, [2]int{10, 20}, [2]int{50, 65}, func(inv *Inventory) { inv.Tools = append(inv.Tools, "wrench") }},
	{"teacher", [2]int{80, 95}, [2]int{20, 35}, [2]int{20, 35}, [2]int{5, 15}, [2]int{60, 75}, func(inv *Inventory) { inv.Special = append(inv.Special, "notebook") }},
	{"police_officer", [2]int{95, 110}, [2]int{25, 40}, [2]int{25, 40}, [2]int{10, 20}, [2]int{50, 65}, func(inv *Inventory) { inv.Weapons = append(inv.Weapons, "baton") }},
	{"farmer", [2]int{90, 105}, [2]int{15, 30}, [2]int{20, 35}, [2]int{5, 15}, [2]int{55, 70}, func(inv *Inventory) { inv.Tools = append(inv.Tools, "multi tool") }},
	{"student", [2]int{75, 90}, [2]int{20, 35}, [2]int{20, 35}, [2]int{5, 15}, [2]int{60, 80}, func(inv *Inventory) { inv.Memento = "photo" }},
}

func pickProfession(r *rand.Rand) professionTemplate {
	return professionTemplates[r.Intn(len(professionTemplates))]
}

func randIn(r *rand.Rand, rng [2]int) int { return rng[0] + r.Intn(rng[1]-rng[0]+1) }

func randomName(r *rand.Rand) string {
	names := []string{"Alex", "Jordan", "Taylor", "Riley", "Morgan", "Casey", "Jamie", "Avery"}
	return names[r.Intn(len(names))]
}

func randomSurname(r *rand.Rand) string { return surnames[r.Intn(len(surnames))] }

// NewFirstSurvivor generates the initial survivor per first-run rule.
// Implements probability 5% researcher pre-outbreak (Day -9..0) inside facility; otherwise day 0 near origin (<=100km) LAD=0.
func NewFirstSurvivor(r *rand.Rand, worldDay int, originRegion string) Survivor {
	// Researcher path decision (5%). If chosen, worldDay random -9..0 and location forced city (facility interior concept), LAD=0 but infected not present until day 0.
	researcher := r.Float64() < 0.05
	if researcher {
		worldDay = -9 + r.Intn(10) // -9..0
	}
	traits := []Trait{AllTraits[r.Intn(len(AllTraits))]}
	for len(traits) < 2 { // ensure two distinct traits
		if t := AllTraits[r.Intn(len(AllTraits))]; t != traits[0] {
			traits = append(traits, t)
		}
	}
	skills := make(map[Skill]int)
	for _, s := range AllSkills {
		skills[s] = 0
	}
	loc := LocationSuburb
	if researcher {
		loc = LocationCity
	}
	// First survivor distance <=100km => Tier A LAD=0
	lad := 0
	prof := pickProfession(r)
	inv := Inventory{Weapons: nil, Ammo: map[string]int{}, FoodDays: 0.5, WaterLiters: 1.0, Medical: []string{"bandage"}, Tools: []string{"pocket knife"}}
	prof.InventoryFn(&inv)
	// Researcher inventory slight variant (lab badge instead of pocket knife replacement)
	if researcher {
		inv.Tools = []string{"lab badge"}
	}
	stats := Stats{Health: randIn(r, prof.HealthRange), Hunger: randIn(r, prof.HungerRange), Thirst: randIn(r, prof.ThirstRange), Fatigue: randIn(r, prof.FatigueRange), Morale: randIn(r, prof.MoraleRange)}
	fullName := randomName(r) + " " + randomSurname(r)
	// Timezone placeholder: derive deterministic from seed via list (simplified until geo mapping added)
	zones := []string{"UTC", "America/New_York", "Europe/London", "Asia/Shanghai", "Europe/Berlin", "America/Chicago"}
	zone := zones[r.Intn(len(zones))]
	return Survivor{
		Name:        fullName,
		Age:         18 + r.Intn(38),
		Background:  prof.Name,
		Region:      originRegion,
		Location:    loc,
		Group:       GroupSolo,
		GroupSize:   1,
		Traits:      traits,
		Skills:      skills,
		Stats:       stats,
		BodyTemp:    TempMild,
		Conditions:  nil,
		Meters:      map[Meter]int{MeterNoise:0,MeterVisibility:0,MeterScent:0,MeterThirstStreak:0,MeterColdExposure:0,MeterFeverRest:0,MeterWarmStreak:0,MeterExhaustionScenes:0,MeterCustomLastTurn:-10},
		Inventory:   inv,
		Environment: Environment{WorldDay: worldDay, TimeOfDay: initialTOD(r), Season: SeasonSpring, Weather: WeatherClear, TempBand: TempMild, Region: originRegion, Location: loc, LAD: lad, Infected: worldDay >= lad, Timezone: zone},
		Alive:       true,
	}
}

// initialTOD returns a simple time-of-day bucket.
func initialTOD(r *rand.Rand) string {
	segments := []string{"pre-dawn", "morning", "midday", "afternoon", "evening", "night"}
	return segments[r.Intn(len(segments))]
}

// NewGenericSurvivor generates a replacement survivor (post-first) using broader randomization.
func NewGenericSurvivor(r *rand.Rand, worldDay int, originRegion string) Survivor {
	traits := []Trait{AllTraits[r.Intn(len(AllTraits))]}
	for len(traits) < 2 {
		if t := AllTraits[r.Intn(len(AllTraits))]; t != traits[0] {
			traits = append(traits, t)
		}
	}
	skills := make(map[Skill]int)
	for _, s := range AllSkills {
		skills[s] = 0
	}
	groups := []GroupType{GroupSolo, GroupDuo, GroupSmallGroup}
	g := groups[r.Intn(len(groups))]
	gSize := 1
	if g == GroupDuo {
		gSize = 2
	} else if g == GroupSmallGroup {
		gSize = 3 + r.Intn(3)
	}
	locs := []LocationType{LocationCity, LocationSuburb, LocationRural}
	loc := locs[r.Intn(len(locs))]
	lad := deriveInitialLAD(r, originRegion, loc, r.Int63())
	prof := pickProfession(r)
	inv := Inventory{Weapons: nil, Ammo: map[string]int{}, FoodDays: 0.4, WaterLiters: 0.8, Medical: []string{"bandage"}}
	prof.InventoryFn(&inv)
	stats := Stats{Health: randIn(r, prof.HealthRange), Hunger: randIn(r, prof.HungerRange), Thirst: randIn(r, prof.ThirstRange), Fatigue: randIn(r, prof.FatigueRange), Morale: randIn(r, prof.MoraleRange)}
	fullName := randomName(r) + " " + randomSurname(r)
	zones := []string{"UTC", "America/New_York", "Europe/London", "Asia/Shanghai", "Europe/Berlin", "America/Chicago", "Australia/Sydney"}
	zone := zones[r.Intn(len(zones))]
	return Survivor{
		Name: fullName, Age: 16 + r.Intn(40), Background: prof.Name, Region: originRegion, Location: loc, Group: g, GroupSize: gSize,
		Traits: traits, Skills: skills, Stats: stats, BodyTemp: TempMild, Conditions: nil, Meters: map[Meter]int{MeterNoise:0,MeterVisibility:0,MeterScent:0,MeterThirstStreak:0,MeterColdExposure:0,MeterFeverRest:0,MeterWarmStreak:0,MeterExhaustionScenes:0,MeterCustomLastTurn:-10}, Inventory: inv,
		Environment: Environment{WorldDay: worldDay, TimeOfDay: initialTOD(r), Season: SeasonSpring, Weather: WeatherClear, TempBand: TempMild, Region: originRegion, Location: loc, LAD: lad, Infected: worldDay >= lad, Timezone: zone}, Alive: true,
	}
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
	if s.IsDead() {
		s.Alive = false
	}
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
		"name":             s.Name,
		"age":              s.Age,
		"background":       s.Background,
		"traits":           s.Traits,
		"skills":           s.Skills,
		"stats":            s.Stats,
		"location":         s.Location,
		"region":           s.Region,
		"world_day":        s.Environment.WorldDay,
		"lad":              s.Environment.LAD,
		"infected_present": s.Environment.Infected,
		"conditions":       s.Conditions,
		"group":            s.Group,
		"group_size":       s.GroupSize,
		"body_temp":        s.BodyTemp,
		"meters":           s.Meters,
		"inventory":        s.Inventory,
		"time_of_day":      s.Environment.TimeOfDay,
		"season":           s.Environment.Season,
		"weather":          s.Environment.Weather,
		"timezone":         s.Environment.Timezone,
		"local_datetime":   narrativeLocalTime(s),
	}
}

func narrativeLocalTime(s Survivor) string {
	// Base reference date (arbitrary stable) 2025-03-01 08:00 UTC
	base := time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC).Add(time.Duration(s.Environment.WorldDay) * 24 * time.Hour)
	// Adjust hour by time-of-day bucket
	switch s.Environment.TimeOfDay {
	case "pre-dawn":
		base = time.Date(base.Year(), base.Month(), base.Day(), 4, 30, 0, 0, time.UTC)
	case "morning":
		base = time.Date(base.Year(), base.Month(), base.Day(), 9, 0, 0, 0, time.UTC)
	case "midday":
		base = time.Date(base.Year(), base.Month(), base.Day(), 12, 30, 0, 0, time.UTC)
	case "afternoon":
		base = time.Date(base.Year(), base.Month(), base.Day(), 15, 30, 0, 0, time.UTC)
	case "evening":
		base = time.Date(base.Year(), base.Month(), base.Day(), 19, 0, 0, 0, time.UTC)
	case "night":
		base = time.Date(base.Year(), base.Month(), base.Day(), 22, 30, 0, 0, time.UTC)
	}
	loc, err := time.LoadLocation(s.Environment.Timezone)
	if err != nil {
		return base.Format(time.RFC3339)
	}
	return base.In(loc).Format(time.RFC3339)
}

// SeedTime returns deterministic time for run day.
func SeedTime(seed int64, day int) time.Time {
	return time.Unix(seed, 0).Add(time.Duration(day) * 24 * time.Hour)
}

// SyncEnvironmentDay updates the environment's world day and infection presence.
func (s *Survivor) SyncEnvironmentDay(day int) {
	s.Environment.WorldDay = day
	s.updateInfectionPresence()
}

func (s *Survivor) updateInfectionPresence() {
	s.Environment.Infected = s.Environment.WorldDay >= s.Environment.LAD
}

// clampSkill restricts skill values to the range 0-5.
func clampSkill(v int) int {
	if v < 0 {
		return 0
	}
	if v > 5 {
		return 5
	}
	return v
}

// NormalizeSkills ensures all skills are within the 0-5 range.
func NormalizeSkills(sk map[Skill]int) {
	for k, v := range sk {
		sk[k] = clampSkill(v)
	}
}

// Survivor skill advancement logic.

func (s *Survivor) GainSkill(sk Skill, meaningful bool) {
	if !meaningful {
		return
	}
	cur := s.Skills[sk]
	if cur < 5 {
		s.Skills[sk] = cur + 1
	}
}
