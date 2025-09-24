package engine

//go:generate go run github.com/DaanHessen/walker-tui/cmd/enumsgen -i internal/engine/enums.yml -o internal/engine/enums.go

// Code generated from enums.yml (manual initial seed). Future: go:generate tool.
// String backed enums for DB interoperability.

type Trait string
type Skill string
type Condition string
type Meter string
type LocationType string
type Season string
type Weather string
type TempBand string
type RiskLevel string
type GroupType string

const (
	TraitCautious    Trait = "cautious"
	TraitImpulsive   Trait = "impulsive"
	TraitStoic       Trait = "stoic"
	TraitEmpathetic  Trait = "empathetic"
	TraitPragmatic   Trait = "pragmatic"
	TraitParanoid    Trait = "paranoid"
	TraitCharismatic Trait = "charismatic"
	TraitLoner       Trait = "loner"
)

var AllTraits = []Trait{TraitCautious, TraitImpulsive, TraitStoic, TraitEmpathetic, TraitPragmatic, TraitParanoid, TraitCharismatic, TraitLoner}

const (
	SkillCombatMelee    Skill = "combat_melee"
	SkillFirearms       Skill = "firearms"
	SkillStealth        Skill = "stealth"
	SkillScavenging     Skill = "scavenging"
	SkillSurvival       Skill = "survival"
	SkillMedicine       Skill = "medicine"
	SkillLeadership     Skill = "leadership"
	SkillTechnical      Skill = "technical"
	SkillCrafting       Skill = "crafting"
	SkillAnimalHandling Skill = "animal_handling"
	SkillDriving        Skill = "driving"
	SkillNavigation     Skill = "navigation"
)

var AllSkills = []Skill{SkillCombatMelee, SkillFirearms, SkillStealth, SkillScavenging, SkillSurvival, SkillMedicine, SkillLeadership, SkillTechnical, SkillCrafting, SkillAnimalHandling, SkillDriving, SkillNavigation}

const (
	ConditionBleeding    Condition = "bleeding"
	ConditionFracture    Condition = "fracture"
	ConditionInfection   Condition = "infection"
	ConditionFever       Condition = "fever"
	ConditionHypothermia Condition = "hypothermia"
	ConditionHeatstroke  Condition = "heatstroke"
	ConditionDehydration Condition = "dehydration"
	ConditionPain        Condition = "pain"
	ConditionPoisoning   Condition = "poisoning"
	ConditionExhaustion  Condition = "exhaustion"
)

var AllConditions = []Condition{ConditionBleeding, ConditionFracture, ConditionInfection, ConditionFever, ConditionHypothermia, ConditionHeatstroke, ConditionDehydration, ConditionPain, ConditionPoisoning, ConditionExhaustion}

const (
	MeterNoise            Meter = "noise"
	MeterVisibility       Meter = "visibility"
	MeterScent            Meter = "scent"
	MeterThirstStreak     Meter = "thirst_streak"
	MeterColdExposure     Meter = "cold_exposure"
	MeterFeverRest        Meter = "fever_rest"
	MeterWarmStreak       Meter = "warm_streak"
	MeterExhaustionScenes Meter = "exhaustion_scenes"
	MeterCustomLastTurn   Meter = "custom_last_turn"
)

var AllMeters = []Meter{MeterNoise, MeterVisibility, MeterScent, MeterThirstStreak, MeterColdExposure, MeterFeverRest, MeterWarmStreak, MeterExhaustionScenes, MeterCustomLastTurn}

const (
	LocationCity     LocationType = "city"
	LocationSuburb   LocationType = "suburb"
	LocationRural    LocationType = "rural"
	LocationForest   LocationType = "forest"
	LocationCoast    LocationType = "coast"
	LocationMountain LocationType = "mountain"
	LocationDesert   LocationType = "desert"
)

var AllLocationTypes = []LocationType{LocationCity, LocationSuburb, LocationRural, LocationForest, LocationCoast, LocationMountain, LocationDesert}

const (
	SeasonSpring Season = "spring"
	SeasonSummer Season = "summer"
	SeasonAutumn Season = "autumn"
	SeasonWinter Season = "winter"
)

var AllSeasons = []Season{SeasonSpring, SeasonSummer, SeasonAutumn, SeasonWinter}

const (
	WeatherClear    Weather = "clear"
	WeatherOvercast Weather = "overcast"
	WeatherRain     Weather = "rain"
	WeatherStorm    Weather = "storm"
	WeatherSnow     Weather = "snow"
	WeatherFog      Weather = "fog"
)

var AllWeather = []Weather{WeatherClear, WeatherOvercast, WeatherRain, WeatherStorm, WeatherSnow, WeatherFog}

const (
	TempFreezing TempBand = "freezing"
	TempCold     TempBand = "cold"
	TempMild     TempBand = "mild"
	TempWarm     TempBand = "warm"
	TempHot      TempBand = "hot"
)

var AllTempBands = []TempBand{TempFreezing, TempCold, TempMild, TempWarm, TempHot}

const (
	RiskLow      RiskLevel = "Low"
	RiskModerate RiskLevel = "Moderate"
	RiskHigh     RiskLevel = "High"
)

var AllRiskLevels = []RiskLevel{RiskLow, RiskModerate, RiskHigh}

const (
	GroupSolo       GroupType = "Solo"
	GroupDuo        GroupType = "Duo"
	GroupSmallGroup GroupType = "SmallGroup"
	GroupCommunity  GroupType = "Community"
)

var AllGroupTypes = []GroupType{GroupSolo, GroupDuo, GroupSmallGroup, GroupCommunity}

// Generic helpers
func contains[T ~string](list []T, v T) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (t Trait) Validate() bool        { return contains(AllTraits, t) }
func (s Skill) Validate() bool        { return contains(AllSkills, s) }
func (c Condition) Validate() bool    { return contains(AllConditions, c) }
func (m Meter) Validate() bool        { return contains(AllMeters, m) }
func (l LocationType) Validate() bool { return contains(AllLocationTypes, l) }
func (s Season) Validate() bool       { return contains(AllSeasons, s) }
func (w Weather) Validate() bool      { return contains(AllWeather, w) }
func (t TempBand) Validate() bool     { return contains(AllTempBands, t) }
func (r RiskLevel) Validate() bool    { return contains(AllRiskLevels, r) }
func (g GroupType) Validate() bool    { return contains(AllGroupTypes, g) }

// List helpers
func ListTraits() []Trait               { return append([]Trait{}, AllTraits...) }
func ListSkills() []Skill               { return append([]Skill{}, AllSkills...) }
func ListConditions() []Condition       { return append([]Condition{}, AllConditions...) }
func ListMeters() []Meter               { return append([]Meter{}, AllMeters...) }
func ListLocationTypes() []LocationType { return append([]LocationType{}, AllLocationTypes...) }
func ListSeasons() []Season             { return append([]Season{}, AllSeasons...) }
func ListWeather() []Weather            { return append([]Weather{}, AllWeather...) }
func ListTempBands() []TempBand         { return append([]TempBand{}, AllTempBands...) }
func ListRiskLevels() []RiskLevel       { return append([]RiskLevel{}, AllRiskLevels...) }
func ListGroupTypes() []GroupType       { return append([]GroupType{}, AllGroupTypes...) }
