package engine

import (
	"math/rand"
)

// Choice mechanical representation (separate from rendered markdown line)
type Choice struct {
	Index     int
	Label     string
	Cost      Cost
	Risk      RiskLevel
	Delta     Stats // simple stat impact for prototype
	Archetype string
}

type Cost struct {
	Time    int // abstract units
	Fatigue int
	Hunger  int
	Thirst  int
}

// Difficulty levels adjust risk probabilities and resource delta scaling.
type Difficulty string

const (
	DifficultyEasy     Difficulty = "easy"
	DifficultyStandard Difficulty = "standard"
	DifficultyHard     Difficulty = "hard"
)

// helper convert RiskLevel to score
func riskScore(r RiskLevel) int {
	switch r {
	case RiskLow:
		return 0
	case RiskModerate:
		return 1
	default:
		return 2
	}
}
func riskFromScore(s int) RiskLevel {
	if s <= 0 {
		return RiskLow
	}
	if s == 1 {
		return RiskModerate
	}
	return RiskHigh
}

// classify archetype category for condition/difficulty modifiers
func archetypeCategory(a string) string {
	switch a {
	case "forage", "travel", "barricade", "scout", "observe":
		return "physical"
	case "organize":
		return "mental"
	case "rest", "pause":
		return "rest"
	default:
		return "general"
	}
}

func relevantSkill(a string) Skill {
	switch a {
	case "forage":
		return SkillScavenging
	case "observe", "scout", "travel":
		return SkillNavigation
	case "organize":
		return SkillLeadership
	case "craft":
		return SkillCrafting
	case "rest", "pause":
		return SkillSurvival
	default:
		return SkillSurvival
	}
}

func adjustRisk(c *Choice, s Survivor, cfg choiceConfig) {
	base := riskScore(c.Risk)
	// skill mod
	sk := relevantSkill(c.Archetype)
	lvl := s.Skills[sk]
	if lvl >= 4 {
		base -= 1
	} else if lvl <= 1 {
		base += 1
	}
	// conditions mods
	cat := archetypeCategory(c.Archetype)
	has := func(cc Condition) bool {
		for _, x := range s.Conditions {
			if x == cc {
				return true
			}
		}
		return false
	}
	if has(ConditionBleeding) && cat == "physical" {
		base += 1
	}
	if has(ConditionDehydration) && (c.Archetype == "forage" || c.Archetype == "travel") {
		base += 1
	}
	if has(ConditionFever) && (c.Archetype == "observe" || c.Archetype == "scout" || c.Archetype == "organize") {
		base += 1
	}
	if has(ConditionHypothermia) && (c.Archetype == "observe" || c.Archetype == "forage") {
		base += 1
	}
	if has(ConditionExhaustion) && cat == "physical" {
		base += 1
	}
	// difficulty mods
	if cfg.difficulty == DifficultyEasy && (c.Archetype == "forage" || c.Archetype == "rest" || c.Archetype == "observe" || c.Archetype == "scout") {
		base -= 1
	}
	if cfg.difficulty == DifficultyHard && (c.Archetype == "forage" || c.Archetype == "travel") {
		base += 1
	}
	if base < 0 {
		base = 0
	}
	if base > 2 {
		base = 2
	}
	c.Risk = riskFromScore(base)
}

// GenerateChoices returns a small deterministic set based on RNG and settings hints.
func GenerateChoices(r *rand.Rand, s Survivor, opts ...ChoiceOption) []Choice {
	cfg := choiceConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	choices := []Choice{
		{Index: 0, Label: "Forage nearby", Archetype: "forage", Cost: Cost{Time: 1, Fatigue: 5}, Risk: RiskLow, Delta: Stats{Hunger: -10, Thirst: -5, Fatigue: 5}},
		{Index: 1, Label: "Rest and recover", Archetype: "rest", Cost: Cost{Time: 1}, Risk: RiskLow, Delta: Stats{Fatigue: -10, Morale: 2}},
	}
	// difficulty scaling pre-adjust (resource yield multipliers)
	resMult := 1.0
	switch cfg.difficulty {
	case DifficultyEasy:
		resMult = 1.2
	case DifficultyHard:
		resMult = 0.75
	}
	choices[0].Delta.Hunger = int(float64(choices[0].Delta.Hunger) * resMult)
	choices[0].Delta.Thirst = int(float64(choices[0].Delta.Thirst) * resMult)
	// Skill influence using existing skills (survival/navigation heuristics)
	if v, ok := s.Skills[SkillSurvival]; ok && v >= 2 {
		choices[0].Delta.Hunger = int(float64(choices[0].Delta.Hunger) * 1.3)
		choices[0].Delta.Thirst = int(float64(choices[0].Delta.Thirst) * 1.25)
	}
	// Additional conditional choices
	if r.Intn(100) < 40 {
		choices = append(choices, Choice{Index: len(choices), Label: "Scout street", Archetype: "observe", Cost: Cost{Time: 1, Fatigue: 8}, Risk: RiskModerate, Delta: Stats{Fatigue: 5, Morale: 3}})
	}
	if v, ok := s.Skills[SkillNavigation]; ok && v >= 2 {
		for i := range choices {
			if choices[i].Label == "Scout street" {
				choices[i].Delta.Morale += 1
			}
		}
	}
	if cfg.textDensity == "rich" && len(choices) < 4 {
		choices = append(choices, Choice{Index: len(choices), Label: "Organize supplies", Archetype: "organize", Cost: Cost{Time: 1}, Risk: RiskLow, Delta: Stats{Morale: 2}})
	}
	// Infection gating – soften risks pre-arrival
	if !cfg.infected {
		for i := range choices {
			if choices[i].Risk == RiskModerate {
				choices[i].Risk = RiskLow
			}
		}
	}
	// Scarcity adjustments
	if cfg.scarcity {
		for i := range choices {
			c := &choices[i]
			if c.Delta.Hunger < 0 {
				c.Delta.Hunger = int(float64(c.Delta.Hunger) * 0.6)
			}
			if c.Delta.Thirst < 0 {
				c.Delta.Thirst = int(float64(c.Delta.Thirst) * 0.6)
			}
			if c.Delta.Fatigue < 0 {
				c.Delta.Fatigue = int(float64(c.Delta.Fatigue) * 0.8)
			}
			c.Cost.Fatigue = int(float64(c.Cost.Fatigue)*1.2 + 0.5)
		}
	}
	// Ensure 2-6 range: pad with a generic low-risk introspection if <2 or add cap
	for len(choices) < 2 {
		choices = append(choices, Choice{Index: len(choices), Label: "Take a cautious pause", Archetype: "pause", Cost: Cost{Time: 1}, Risk: RiskLow, Delta: Stats{Morale: 1}})
	}
	if len(choices) > 6 {
		choices = choices[:6]
	}
	// Reindex to be safe
	for i := range choices {
		choices[i].Index = i
	}
	// risk recalculation pass
	for i := range choices {
		adjustRisk(&choices[i], s, cfg)
	}
	return choices
}

// ApplyChoice applies mechanical deltas and returns resulting stats delta.
func ApplyChoice(s *Survivor, c Choice, diff Difficulty, currentTurn int) Stats {
	d := c.Delta
	// Convert cost to drains
	d.Fatigue += c.Cost.Fatigue
	d.Hunger += c.Cost.Hunger
	d.Thirst += c.Cost.Thirst
	// baseline drains by difficulty
	baseH, baseT, baseF := 2, 3, 2 // standard defaults
	switch diff {
	case DifficultyEasy:
		baseH, baseT, baseF = 1, 2, 1
	case DifficultyHard:
		baseH, baseT, baseF = 3, 4, 3
	}
	d.Hunger += baseH
	d.Thirst += baseT
	d.Fatigue += baseF
	s.UpdateStats(d)
	advanceConditions(s)
	// remove duplicate old Tick baseline to prevent double drain
	// s.Tick()
	s.EvaluateDeath()
	if c.Index == -1 && s.Meters != nil {
		// record last custom action turn for cooldown enforcement
		s.Meters[MeterCustomLastTurn] = currentTurn
	}
	if c.Archetype != "" {
		s.GainSkill(relevantSkill(c.Archetype), true)
	}
	return d
}

func advanceConditions(s *Survivor) {
	if s.Meters == nil {
		return
	}
	// thirst streak for dehydration trigger
	if s.Stats.Thirst >= 80 {
		s.Meters[MeterThirstStreak]++
	} else {
		s.Meters[MeterThirstStreak] = 0
	}
	// cold exposure
	if s.Environment.TempBand == TempCold || s.Environment.TempBand == TempFreezing {
		s.Meters[MeterColdExposure]++
	} else {
		s.Meters[MeterColdExposure] = 0
	}
	// fever rest tracking if fever present and resting archetype chosen handled externally; placeholder decrement
	if hasCondition(s, ConditionFever) {
		s.Meters[MeterFeverRest]++
	} else {
		s.Meters[MeterFeverRest] = 0
	}
	// warm streak for hypothermia recovery
	if !(s.Environment.TempBand == TempCold || s.Environment.TempBand == TempFreezing) {
		s.Meters[MeterWarmStreak]++
	} else {
		s.Meters[MeterWarmStreak] = 0
	}
	// exhaustion scenes counter
	if s.Stats.Fatigue >= 85 {
		s.Meters[MeterExhaustionScenes]++
	} else {
		s.Meters[MeterExhaustionScenes] = 0
	}
	applyConditionTransitions(s)
	applyConditionDrains(s)
}

func hasCondition(s *Survivor, c Condition) bool {
	for _, x := range s.Conditions {
		if x == c {
			return true
		}
	}
	return false
}
func addCondition(s *Survivor, c Condition) {
	if hasCondition(s, c) {
		return
	}
	s.Conditions = append(s.Conditions, c)
}
func removeCondition(s *Survivor, c Condition) {
	out := make([]Condition, 0, len(s.Conditions))
	for _, x := range s.Conditions {
		if x != c {
			out = append(out, x)
		}
	}
	s.Conditions = out
}

func applyConditionTransitions(s *Survivor) {
	if s.Meters[MeterThirstStreak] >= 3 {
		addCondition(s, ConditionDehydration)
	}
	if s.Meters[MeterColdExposure] >= 3 {
		addCondition(s, ConditionHypothermia)
	}
	if s.Meters[MeterExhaustionScenes] >= 1 {
		addCondition(s, ConditionExhaustion)
	}
	// recovery
	if s.Meters[MeterThirstStreak] == 0 && s.Stats.Thirst <= 40 {
		removeCondition(s, ConditionDehydration)
	}
	if s.Meters[MeterWarmStreak] >= 4 {
		removeCondition(s, ConditionHypothermia)
	}
	if s.Stats.Fatigue <= 50 {
		removeCondition(s, ConditionExhaustion)
	}
}

func applyConditionDrains(s *Survivor) {
	for _, c := range s.Conditions {
		switch c {
		case ConditionBleeding:
			s.UpdateStats(Stats{Health: -6, Fatigue: 2})
		case ConditionDehydration:
			s.UpdateStats(Stats{Fatigue: 2, Morale: -2, Health: -1})
		case ConditionFever:
			s.UpdateStats(Stats{Fatigue: 1, Morale: -2, Health: -1})
		case ConditionHypothermia:
			s.UpdateStats(Stats{Health: -3, Fatigue: 2})
		case ConditionExhaustion:
			if s.Meters[MeterExhaustionScenes] >= 4 {
				s.UpdateStats(Stats{Health: -1})
			}
		}
	}
}

// ChoiceOption functional options to pass runtime settings.
type ChoiceOption func(*choiceConfig)

type choiceConfig struct {
	scarcity    bool
	textDensity string
	infected    bool
	difficulty  Difficulty
}

func WithScarcity(b bool) ChoiceOption         { return func(c *choiceConfig) { c.scarcity = b } }
func WithTextDensity(d string) ChoiceOption    { return func(c *choiceConfig) { c.textDensity = d } }
func WithInfectedPresent(b bool) ChoiceOption  { return func(c *choiceConfig) { c.infected = b } }
func WithDifficulty(d Difficulty) ChoiceOption { return func(c *choiceConfig) { c.difficulty = d } }
