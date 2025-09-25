package engine

import "fmt"

// Choice mechanical representation (separate from rendered markdown line)
type StatKey string

const (
	StatHealth  StatKey = "health"
	StatHunger  StatKey = "hunger"
	StatThirst  StatKey = "thirst"
	StatFatigue StatKey = "fatigue"
	StatMorale  StatKey = "morale"
)

type DeltaRange struct {
	Min int
	Max int
}

type ChoiceOutcome map[StatKey]DeltaRange

type ChoiceEffect struct {
	AddConditions    []Condition
	RemoveConditions []Condition
	MeterDeltas      map[Meter]int
}

func (e ChoiceEffect) Empty() bool {
	return len(e.AddConditions) == 0 && len(e.RemoveConditions) == 0 && len(e.MeterDeltas) == 0
}

func mergeEffects(a, b ChoiceEffect) ChoiceEffect {
	res := ChoiceEffect{}
	res.AddConditions = append(res.AddConditions, a.AddConditions...)
	res.AddConditions = append(res.AddConditions, b.AddConditions...)
	res.RemoveConditions = append(res.RemoveConditions, a.RemoveConditions...)
	res.RemoveConditions = append(res.RemoveConditions, b.RemoveConditions...)
	if len(a.MeterDeltas) > 0 || len(b.MeterDeltas) > 0 {
		res.MeterDeltas = make(map[Meter]int, len(a.MeterDeltas)+len(b.MeterDeltas))
		for k, v := range a.MeterDeltas {
			res.MeterDeltas[k] += v
		}
		for k, v := range b.MeterDeltas {
			res.MeterDeltas[k] += v
		}
	}
	return res
}

func applyChoiceEffect(s *Survivor, eff ChoiceEffect) (added []Condition, removed []Condition) {
	if s == nil || eff.Empty() {
		return nil, nil
	}
	if s.Meters == nil {
		s.Meters = make(map[Meter]int)
	}
	for _, cond := range eff.AddConditions {
		if addConditionIfAbsent(s, cond) {
			added = append(added, cond)
		}
	}
	for _, cond := range eff.RemoveConditions {
		if removeConditionIfPresent(s, cond) {
			removed = append(removed, cond)
		}
	}
	if len(eff.MeterDeltas) > 0 {
		for meter, delta := range eff.MeterDeltas {
			v := s.Meters[meter] + delta
			if v < 0 {
				v = 0
			}
			if v > 100 {
				v = 100
			}
			s.Meters[meter] = v
		}
	}
	return added, removed
}

func survivorHasCondition(s Survivor, cond Condition) bool {
	for _, existing := range s.Conditions {
		if existing == cond {
			return true
		}
	}
	return false
}

func addConditionIfAbsent(s *Survivor, cond Condition) bool {
	if s == nil || survivorHasCondition(*s, cond) {
		return false
	}
	s.Conditions = append(s.Conditions, cond)
	return true
}

func removeConditionIfPresent(s *Survivor, cond Condition) bool {
	if s == nil {
		return false
	}
	if len(s.Conditions) == 0 {
		return false
	}
	rest := s.Conditions[:0]
	removed := false
	for _, existing := range s.Conditions {
		if existing == cond {
			removed = true
			continue
		}
		rest = append(rest, existing)
	}
	s.Conditions = rest
	return removed
}

type Choice struct {
	Index       int
	ID          string
	Label       string
	Cost        Cost
	Risk        RiskLevel
	Archetype   string
	Outcome     ChoiceOutcome
	Effects     ChoiceEffect
	SourceEvent string
	Custom      bool
}

type Resolution struct {
	Delta   Stats
	Added   []Condition
	Removed []Condition
}

type conditionOutcome struct {
	Delta   Stats
	Added   []Condition
	Removed []Condition
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

func isHighExertionChoice(c Choice) bool {
	if c.Archetype == "rest" || c.Archetype == "organize" {
		return false
	}
	if c.Cost.Fatigue >= 6 {
		return true
	}
	switch c.Archetype {
	case "forage", "travel", "barricade", "scout":
		return true
	default:
		return false
	}
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
	sk := relevantSkill(c.Archetype)
	lvl := s.Skills[sk]
	if lvl >= 4 {
		base--
	} else if lvl <= 1 {
		base++
	}
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
		base++
	}
	if has(ConditionFever) && (c.Archetype == "observe" || c.Archetype == "scout") {
		base++
	}
	if has(ConditionHypothermia) && (c.Archetype == "forage" || c.Archetype == "scout" || c.Archetype == "barricade") {
		base++
	}
	if has(ConditionExhaustion) && cat == "physical" {
		base++
	}
	if cfg.difficulty == DifficultyEasy && (c.Archetype == "forage" || c.Archetype == "rest" || c.Archetype == "scout") {
		base--
	}
    if cfg.difficulty == DifficultyHard && isHighExertionChoice(*c) {
        base++
    }
    // Progressive infected pressure post-arrival
    if s.Environment.WorldDay >= s.Environment.LAD {
        days := s.Environment.WorldDay - s.Environment.LAD
        if days >= 14 && c.Archetype != "rest" {
            base++
        }
    }
    if base < 0 {
        base = 0
    }
	if base > 2 {
		base = 2
	}
	c.Risk = riskFromScore(base)
}

// GenerateChoices returns a deterministic set based on RNG and settings hints.
func GenerateChoices(stream *Stream, s *Survivor, history EventHistory, sceneIdx int, opts ...ChoiceOption) ([]Choice, *EventContext, error) {
	cfg := choiceConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	ctx, onSelectEffect, err := SelectAndBuildChoices(stream.Child("event-selection"), s, cfg, history, sceneIdx)
	if err != nil {
		return nil, nil, err
	}
	if !onSelectEffect.Empty() {
		_, _ = applyChoiceEffect(s, onSelectEffect)
	}
	choices := make([]Choice, len(ctx.Choices))
	copy(choices, ctx.Choices)
	for i := range choices {
		adjustRisk(&choices[i], *s, cfg)
	}
	return choices, ctx, nil
}

// ApplyChoice applies mechanical deltas and returns resulting resolution summary.
func ApplyChoice(s *Survivor, c Choice, diff Difficulty, currentTurn int, randStream *Stream) Resolution {
	result := Resolution{}
	if s.Meters == nil {
		s.Meters = make(map[Meter]int)
	}
	statStream := randStream
	if statStream == nil {
		seed := Derive(SeedFromString(c.ID), fmt.Sprintf("turn:%d", currentTurn))
		statStream = newStream(seed)
	}
	delta := sampleOutcome(c.Outcome, statStream)
	delta.Fatigue += c.Cost.Fatigue
	delta.Hunger += c.Cost.Hunger
	delta.Thirst += c.Cost.Thirst
	baseH, baseT, baseF := 2, 3, 2
	switch diff {
	case DifficultyEasy:
		baseH, baseT, baseF = 1, 2, 1
	case DifficultyHard:
		baseH, baseT, baseF = 3, 4, 3
	}
	delta.Hunger += baseH
	delta.Thirst += baseT
	delta.Fatigue += baseF
	s.UpdateStats(delta)
	added, removed := applyChoiceEffect(s, c.Effects)
	if len(added) > 0 {
		result.Added = append(result.Added, added...)
	}
	if len(removed) > 0 {
		result.Removed = append(result.Removed, removed...)
	}
	condOutcome := advanceConditions(s, diff, c, delta)
	if condOutcome.Delta != (Stats{}) {
		s.UpdateStats(condOutcome.Delta)
		delta = addStats(delta, condOutcome.Delta)
	}
	if len(condOutcome.Added) > 0 {
		result.Added = append(result.Added, condOutcome.Added...)
	}
	if len(condOutcome.Removed) > 0 {
		result.Removed = append(result.Removed, condOutcome.Removed...)
	}
	s.EvaluateDeath()
	if c.Index == -1 {
		s.Meters[MeterCustomLastTurn] = currentTurn
	}
	if c.Archetype != "" {
		s.GainSkill(relevantSkill(c.Archetype), true)
	}
	result.Delta = delta
	return result
}

func sampleOutcome(out ChoiceOutcome, stream *Stream) Stats {
	res := Stats{}
	if len(out) == 0 {
		return res
	}
	for key, rng := range out {
		label := "stat:" + string(key)
		value := sampleValue(rng, stream, label)
		switch key {
		case StatHealth:
			res.Health += value
		case StatHunger:
			res.Hunger += value
		case StatThirst:
			res.Thirst += value
		case StatFatigue:
			res.Fatigue += value
		case StatMorale:
			res.Morale += value
		}
	}
	return res
}

func sampleValue(rng DeltaRange, stream *Stream, label string) int {
	minV := rng.Min
	maxV := rng.Max
	if stream == nil || minV == maxV {
		return minV
	}
	span := maxV - minV + 1
	if span <= 1 {
		return minV
	}
	local := stream.Child(label)
	return minV + local.Intn(span)
}

func addStats(a, b Stats) Stats {
	return Stats{
		Health:  a.Health + b.Health,
		Hunger:  a.Hunger + b.Hunger,
		Thirst:  a.Thirst + b.Thirst,
		Fatigue: a.Fatigue + b.Fatigue,
		Morale:  a.Morale + b.Morale,
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
