package engine

import (
    "strings"
    "time"
)

// World holds run-wide data.
type World struct {
	OriginSite   string
	Seed         RunSeed
	RulesVersion string
	CurrentDay   int // advances globally; survivors spawn into this
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
    DistanceToOriginKM float64 // optional; first survivor ~<=100km; not displayed
}

// ComputeLAD calculates Local Arrival Day based on distance and modifiers.
func ComputeLAD(distanceKM float64, hub bool, rural bool, closures bool, evac bool, jitterStream *Stream) int {
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
    // start at midpoint
    lad := (baseMin + baseMax) / 2
    // hub airport / HSR: -2 (min Day 0 for B+ tiers)
    if hub {
        lad -= 2
        if baseMin > 0 && lad < 0 { // never below day 0 for non-TierA
            lad = 0
        }
    }
    // rural: +2..+5
    if rural {
        lad += 2 + jitterStream.Child("rural").Intn(4) // 2..5
    }
    // border closures: +2..+7
    if closures {
        lad += 2 + jitterStream.Child("closures").Intn(6) // 2..7
    }
    // evacuation routes: -2..-1
    if evac {
        lad -= 2 - jitterStream.Child("evac").Intn(2) // -2 or -1
        if baseMin > 0 && lad < 0 {
            lad = 0
        }
    }
    if lad < baseMin {
        lad = baseMin
    }
    if lad > baseMax {
        lad = baseMax
    }
    return lad
}

func deriveInitialLAD(stream *Stream, loc LocationType) int {
	// Distance heuristics per location tier
	distStream := stream.Child("distance")
	var distance float64
	switch loc {
	case LocationCity:
		distance = 50 + distStream.Float64()*150 // 50-200
	case LocationSuburb:
		distance = 150 + distStream.Float64()*600 // 150-750
	case LocationRural:
		distance = 400 + distStream.Float64()*2600 // 400-3000
	default:
		distance = 500 + distStream.Float64()*2000
	}
	hub := loc == LocationCity
	ruralFlag := loc == LocationRural
	closures := distStream.Child("closures").Float64() < 0.15
	evac := distStream.Child("evac").Float64() < 0.20
	return ComputeLAD(distance, hub, ruralFlag, closures, evac, stream.Child("lad"))
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

func pickProfession(stream *Stream) professionTemplate {
	return professionTemplates[stream.Child("profession").Intn(len(professionTemplates))]
}

func randIn(stream *Stream, rng [2]int) int {
	span := rng[1] - rng[0] + 1
	if span <= 0 {
		return rng[0]
	}
	return rng[0] + stream.Intn(span)
}

func randomName(stream *Stream) string {
	names := []string{"Alex", "Jordan", "Taylor", "Riley", "Morgan", "Casey", "Jamie", "Avery"}
	return names[stream.Child("given").Intn(len(names))]
}

func randomSurname(stream *Stream) string {
	return surnames[stream.Child("surname").Intn(len(surnames))]
}

// NewFirstSurvivor generates the initial survivor per first-run rule.
func NewFirstSurvivor(stream *Stream, originRegion string) Survivor {
	roleStream := stream.Child("role")
	researcher := roleStream.Float64() < 0.05
	worldDay := 0
	if researcher {
		worldDay = -9 + roleStream.Child("researcher-day").Intn(10)
	}

	traits := selectTraits(stream.Child("traits"), 2)
	skills := baselineSkills()
	loc := LocationSuburb
	if researcher {
		loc = LocationCity
	}
	lad := 0 // Tier A per spec

	prof := pickProfession(stream.Child("profession"))
	inv := baseInventory(stream.Child("inventory"))
	prof.InventoryFn(&inv)
	if researcher {
		inv.Tools = []string{"lab badge"}
	}

	statsStream := stream.Child("stats")
	stats := Stats{
		Health:  randIn(statsStream.Child("health"), prof.HealthRange),
		Hunger:  randIn(statsStream.Child("hunger"), prof.HungerRange),
		Thirst:  randIn(statsStream.Child("thirst"), prof.ThirstRange),
		Fatigue: randIn(statsStream.Child("fatigue"), prof.FatigueRange),
		Morale:  randIn(statsStream.Child("morale"), prof.MoraleRange),
	}

	nameStream := stream.Child("name")
	fullName := randomName(nameStream) + " " + randomSurname(nameStream)
	zoneStream := stream.Child("timezone")
	zones := []string{"UTC", "America/New_York", "Europe/London", "Asia/Shanghai", "Europe/Berlin", "America/Chicago"}
	zone := zones[zoneStream.Intn(len(zones))]

    // derive a non-revealing region label for UI
    regionLabel := generalRegion(originRegion, stream.Child("region-label"))
    env := Environment{
        WorldDay:  worldDay,
        TimeOfDay: initialTOD(stream.Child("tod")),
        Season:    SeasonSpring,
        Weather:   WeatherClear,
        TempBand:  TempMild,
        Region:    regionLabel,
        Location:  loc,
        LAD:       lad,
        Infected:  worldDay >= lad,
        Timezone:  zone,
        DistanceToOriginKM: stream.Child("origin-distance").Float64()*100.0, // 0..100
    }

    survivor := Survivor{
		Name:        fullName,
		Age:         18 + stream.Child("age").Intn(38),
		Background:  prof.Name,
        Region:      regionLabel,
		Location:    loc,
		Group:       GroupSolo,
		GroupSize:   1,
		Traits:      traits,
		Skills:      skills,
		Stats:       stats,
		BodyTemp:    TempMild,
		Conditions:  nil,
		Meters:      baselineMeters(),
		Inventory:   inv,
		Environment: env,
		Alive:       true,
	}
	return survivor
}

func initialTOD(stream *Stream) string {
	segments := []string{"pre-dawn", "morning", "midday", "afternoon", "evening", "night"}
	return segments[stream.Intn(len(segments))]
}

// NewGenericSurvivor generates a replacement survivor using broader randomization.
func NewGenericSurvivor(stream *Stream, worldDay int, originRegion string) Survivor {
    traits := selectTraits(stream.Child("traits"), 2)
    skills := baselineSkills()
	groupStream := stream.Child("group")
	groups := []GroupType{GroupSolo, GroupDuo, GroupSmallGroup}
	g := groups[groupStream.Intn(len(groups))]
	gSize := 1
	switch g {
	case GroupDuo:
		gSize = 2
	case GroupSmallGroup:
		gSize = 3 + groupStream.Child("size").Intn(3)
	}

	locs := []LocationType{LocationCity, LocationSuburb, LocationRural}
	loc := locs[stream.Child("location").Intn(len(locs))]
	lad := deriveInitialLAD(stream.Child("lad"), loc)

	prof := pickProfession(stream.Child("profession"))
	inv := baseInventory(stream.Child("inventory"))
	prof.InventoryFn(&inv)

	statsStream := stream.Child("stats")
	stats := Stats{
		Health:  randIn(statsStream.Child("health"), prof.HealthRange),
		Hunger:  randIn(statsStream.Child("hunger"), prof.HungerRange),
		Thirst:  randIn(statsStream.Child("thirst"), prof.ThirstRange),
		Fatigue: randIn(statsStream.Child("fatigue"), prof.FatigueRange),
		Morale:  randIn(statsStream.Child("morale"), prof.MoraleRange),
	}

    nameStream := stream.Child("name")
    fullName := randomName(nameStream) + " " + randomSurname(nameStream)
    zones := []string{"UTC", "America/New_York", "Europe/London", "Asia/Shanghai", "Europe/Berlin", "America/Chicago", "Australia/Sydney"}
    zone := zones[stream.Child("timezone").Intn(len(zones))]
    // generic survivors may be anywhere in the world
    regionLabel := pickWorldRegion(stream.Child("world-region"))

	survivor := Survivor{
		Name:       fullName,
		Age:        16 + stream.Child("age").Intn(40),
		Background: prof.Name,
        Region:     regionLabel,
		Location:   loc,
		Group:      g,
		GroupSize:  gSize,
		Traits:     traits,
		Skills:     skills,
		Stats:      stats,
		BodyTemp:   TempMild,
		Conditions: nil,
		Meters:     baselineMeters(),
		Inventory:  inv,
		Environment: Environment{
			WorldDay:  worldDay,
			TimeOfDay: initialTOD(stream.Child("tod")),
			Season:    SeasonSpring,
			Weather:   WeatherClear,
			TempBand:  TempMild,
			Region:    originRegion,
			Location:  loc,
			LAD:       lad,
			Infected:  worldDay >= lad,
			Timezone:  zone,
		},
		Alive: true,
	}
	return survivor
}

func selectTraits(stream *Stream, count int) []Trait {
	if count <= 0 {
		return nil
	}
	traits := make([]Trait, 0, count)
	pool := append([]Trait{}, AllTraits...)
	for len(traits) < count && len(pool) > 0 {
		idx := stream.Intn(len(pool))
		traits = append(traits, pool[idx])
		pool = append(pool[:idx], pool[idx+1:]...)
	}
	return traits
}

func baselineSkills() map[Skill]int {
	skills := make(map[Skill]int, len(AllSkills))
	for _, s := range AllSkills {
		skills[s] = 0
	}
	return skills
}

func baselineMeters() map[Meter]int {
	return map[Meter]int{
		MeterNoise:             0,
		MeterVisibility:        0,
		MeterScent:             0,
		MeterThirstStreak:      0,
		MeterHydrationRecovery: 0,
		MeterColdExposure:      0,
		MeterFeverRest:         0,
		MeterFeverMedication:   0,
		MeterWarmStreak:        0,
		MeterExhaustionScenes:  0,
		MeterCustomLastTurn:    -10,
	}
}

func baseInventory(stream *Stream) Inventory {
	return Inventory{
		Weapons:     nil,
		Ammo:        map[string]int{},
		FoodDays:    0.5,
		WaterLiters: 1.0,
		Medical:     []string{"bandage"},
		Tools:       []string{"pocket knife"},
	}
}

// generalRegion maps a hidden origin site string to a non-revealing coarse region label.
func generalRegion(origin string, stream *Stream) string {
    // Attempt to read country from parentheses
    country := ""
    if i := strings.LastIndex(origin, "("); i >= 0 && strings.HasSuffix(origin, ")") {
        country = strings.TrimSuffix(origin[i+1:], ")")
    }
    switch country {
    case "USA":
        labs := []string{"Mid-Atlantic, USA", "Gulf Coast, USA", "Midwest, USA"}
        return labs[stream.Intn(len(labs))]
    case "UK":
        opts := []string{"Southern England, UK", "Western England, UK"}
        return opts[stream.Intn(len(opts))]
    case "Germany":
        opts := []string{"Northern Germany", "Western Germany"}
        return opts[stream.Intn(len(opts))]
    case "China":
        opts := []string{"Central China", "Eastern China"}
        return opts[stream.Intn(len(opts))]
    case "Russia":
        opts := []string{"Western Russia"}
        return opts[stream.Intn(len(opts))]
    default:
        // Fallback coarse label
        return "Unknown Region"
    }
}

// pickWorldRegion selects a broad world region label for replacement survivors.
func pickWorldRegion(stream *Stream) string {
    regions := []string{
        "Northeast USA", "West Coast USA", "Great Plains USA",
        "Western Europe", "Northern Europe", "Southern Europe",
        "Eastern Europe", "Central China", "Eastern China", "South Asia",
        "Southeast Asia", "Oceania", "South America", "North Africa",
    }
    return regions[stream.Intn(len(regions))]
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

// IsDead returns if survivor is dead.
func (s *Survivor) IsDead() bool { return s.Stats.Health <= 0 }

// EvaluateDeath toggles Alive based on health.
func (s *Survivor) EvaluateDeath() {
	if s.IsDead() {
		s.Alive = false
	}
}

// NewWorld initialises world data using deterministic seeding.
func NewWorld(seed RunSeed, rulesVersion string) *World {
	origin := pickOrigin(seed.Stream("origin@rules:" + rulesVersion))
	return &World{OriginSite: origin, Seed: seed, RulesVersion: rulesVersion, CurrentDay: 0}
}

func pickOrigin(stream *Stream) string {
	sites := []string{
		"USAMRIID/Fort Detrick (USA)",
		"Galveston National Lab (USA)",
		"Porton Down (UK)",
		"Vector Institute (Russia)",
		"Riems Island Lab (Germany)",
		"Wuhan Institute of Virology (China)",
	}
	return sites[stream.Intn(len(sites))]
}

// NarrativeState collects survivor info for narration/UI.
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
	base := time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC).Add(time.Duration(s.Environment.WorldDay) * 24 * time.Hour)
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

// NarrativeLocalTime returns the local date-time string used in UI top bar.
func NarrativeLocalTime(s Survivor) string { return narrativeLocalTime(s) }

// SeedTime returns deterministic time for run day.
func SeedTime(seed RunSeed, day int) time.Time {
	// Use lower 63 bits to keep within int64 range.
	unix := int64(seed.root & 0x7FFFFFFFFFFFFFFF)
	return time.Unix(unix, 0).Add(time.Duration(day) * 24 * time.Hour)
}

// SyncEnvironmentDay updates the environment's world day and infection presence.
func (s *Survivor) SyncEnvironmentDay(day int) {
	s.Environment.WorldDay = day
	s.updateInfectionPresence()
}

func (s *Survivor) updateInfectionPresence() {
	s.Environment.Infected = s.Environment.WorldDay >= s.Environment.LAD
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
