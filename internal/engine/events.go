package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Event struct {
	ID              string            `yaml:"id"`
	Name            string            `yaml:"name"`
	Tags            []string          `yaml:"tags"`
	Tier            string            `yaml:"tier"`
	Rarity          string            `yaml:"rarity"`
	CooldownScenes  int               `yaml:"cooldown_scenes"`
	OncePerRun      bool              `yaml:"once_per_run"`
	Pre             EventPre          `yaml:"preconditions"`
	EffectsOnSelect effectSpec        `yaml:"effects_on_select"`
	Choices         []EventChoice     `yaml:"choices"`
	OutcomeDeltas   map[string]string `yaml:"outcome_deltas"`
	Arc             *EventArc         `yaml:"arc"`
}

type EventPre struct {
	LADRelation   string         `yaml:"lad_relation"`
	LocationTypes []string       `yaml:"location_types"`
	TimeOfDay     []string       `yaml:"time_of_day"`
	MinDay        *int           `yaml:"min_day"`
	MaxDay        *int           `yaml:"max_day"`
	ForbidConds   []string       `yaml:"forbid_conditions"`
	RequireConds  []string       `yaml:"require_conditions"`
	SkillsAnyMin  map[string]int `yaml:"skills_any_min"`
}

type EventChoice struct {
	ID         string            `yaml:"id"`
	Label      string            `yaml:"label"`
	Archetypes []string          `yaml:"archetypes"`
	BaseRisk   string            `yaml:"base_risk"`
	BaseCost   EventCostSpec     `yaml:"base_cost"`
	Gating     ChoiceGating      `yaml:"gating"`
	Outcome    map[string]string `yaml:"outcome_deltas"`
	Effects    effectSpec        `yaml:"effects_on_resolve"`
}

type EventCostSpec struct {
	TimeMin    int `yaml:"time_min"`
	Fatigue    int `yaml:"fatigue"`
	Hunger     int `yaml:"hunger"`
	Thirst     int `yaml:"thirst"`
	Noise      int `yaml:"noise"`
	Visibility int `yaml:"visibility"`
}

type ChoiceGating struct {
	RequireConditions []string       `yaml:"require_conditions"`
	ForbidConditions  []string       `yaml:"forbid_conditions"`
	SkillsMin         map[string]int `yaml:"skills_min"`
}

type effectSpec struct {
	AddConditions    []Condition    `yaml:"add_conditions"`
	RemoveConditions []Condition    `yaml:"remove_conditions"`
	Meters           map[string]int `yaml:"meters"`
}

func (e effectSpec) toChoiceEffect() (ChoiceEffect, error) {
	effect := ChoiceEffect{}
	if len(e.AddConditions) > 0 {
		effect.AddConditions = append(effect.AddConditions, e.AddConditions...)
	}
	if len(e.RemoveConditions) > 0 {
		effect.RemoveConditions = append(effect.RemoveConditions, e.RemoveConditions...)
	}
	if len(e.Meters) > 0 {
		effect.MeterDeltas = make(map[Meter]int, len(e.Meters))
		for raw, delta := range e.Meters {
			meter := Meter(strings.TrimSpace(raw))
			if !meter.Validate() {
				return ChoiceEffect{}, fmt.Errorf("unknown meter %s in effect", raw)
			}
			effect.MeterDeltas[meter] += delta
		}
	}
	return effect, nil
}

type EventArc struct {
	ID                 string   `yaml:"id"`
	Step               int      `yaml:"step"`
	NextMinDelayScenes int      `yaml:"next_min_delay_scenes"`
	NextCandidates     []string `yaml:"next_candidates"`
}

type EventContext struct {
	Event   Event
	Choices []Choice
}

type EventState struct {
	LastSceneIdx       int
	CooldownUntilScene int
	OnceFired          bool
}

type ArcState struct {
	LastStep     int
	LastSceneIdx int
	LastEventID  string
}

type EventHistory struct {
	Events map[string]EventState
	Arcs   map[string]ArcState
}

func (h EventHistory) eventState(id string) EventState {
	if h.Events == nil {
		return EventState{}
	}
	return h.Events[id]
}

func (h EventHistory) arcState(id string) (ArcState, bool) {
	if id == "" || h.Arcs == nil {
		return ArcState{}, false
	}
	arc, ok := h.Arcs[id]
	return arc, ok
}

var (
	cachedEvents []Event
	eventsOnce   sync.Once
	eventsErr    error
	// testEventsOverride allows tests to inject a custom event set.
	// When non-nil and returns ok=true, loadEvents will return the provided events.
	testEventsOverride func() ([]Event, bool)
)

func loadEvents(dir string) ([]Event, error) {
    // Test override hook: if present, bypass on-disk loading.
    if testEventsOverride != nil {
        if evs, ok := testEventsOverride(); ok {
            return evs, nil
        }
    }
    eventsOnce.Do(func() {
        var out []Event
        // Try several candidate roots so tests from subpackages find assets.
        roots := []string{
            dir,
            filepath.Join("..", "..", dir),
            filepath.Join("..", dir),
        }
        var walkErr error
        var found bool
        for _, root := range roots {
            walkErr = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
                if err != nil {
                    return err
                }
                if info.IsDir() {
                    return nil
                }
                ext := strings.ToLower(filepath.Ext(path))
                if ext != ".yaml" && ext != ".yml" {
                    return nil
                }
                data, err := os.ReadFile(path)
                if err != nil {
                    return err
                }
                var ev Event
                if err := yaml.Unmarshal(data, &ev); err != nil {
                    return fmt.Errorf("%s: %w", path, err)
                }
                if ev.ID == "" {
                    return fmt.Errorf("%s: missing id", path)
                }
                if len(ev.Choices) == 0 {
                    return fmt.Errorf("%s: event %s has no choices", path, ev.ID)
                }
                out = append(out, ev)
                found = true
                return nil
            })
            if walkErr == nil && found {
                break
            }
        }
        if walkErr != nil {
            eventsErr = walkErr
            return
        }
        if len(out) == 0 {
            eventsErr = errors.New("no events found in assets/events")
            return
        }
        cachedEvents = out
    })
    return cachedEvents, eventsErr
}

func rarityWeight(r string) int {
	switch strings.ToLower(r) {
	case "rare":
		return 1
	case "uncommon":
		return 3
	default:
		return 5
	}
}

func SelectAndBuildChoices(stream *Stream, s *Survivor, cfg choiceConfig, history EventHistory, sceneIdx int) (*EventContext, ChoiceEffect, error) {
	events, err := loadEvents("assets/events")
	if err != nil {
		return nil, ChoiceEffect{}, err
	}
	eventsByID := make(map[string]Event, len(events))
	for _, ev := range events {
		eventsByID[ev.ID] = ev
	}
	type candidate struct {
		event   Event
		choices []Choice
		weight  int
	}
	candidates := make([]candidate, 0, len(events))
	preArrival := s.Environment.WorldDay < s.Environment.LAD
	for _, ev := range events {
		if !tierMatches(ev.Tier, preArrival, *s) {
			continue
		}
		if !eventPreconditionsSatisfied(ev.Pre, *s) {
			continue
		}
		state := history.eventState(ev.ID)
		if ev.OncePerRun && state.OnceFired {
			continue
		}
		if state.CooldownUntilScene > sceneIdx {
			continue
		}
		if ev.Arc != nil {
			arcState, ok := history.arcState(ev.Arc.ID)
			step := ev.Arc.Step
			if step <= 0 {
				step = 1
			}
			if step > 1 {
				if !ok || arcState.LastStep != step-1 {
					continue
				}
				if sceneIdx-arcState.LastSceneIdx < ev.Arc.NextMinDelayScenes {
					continue
				}
				if prev, okPrev := eventsByID[arcState.LastEventID]; okPrev && prev.Arc != nil && len(prev.Arc.NextCandidates) > 0 {
					allowed := false
					for _, nextID := range prev.Arc.NextCandidates {
						if nextID == ev.ID {
							allowed = true
							break
						}
					}
					if !allowed {
						continue
					}
				}
			} else {
				if ok && arcState.LastStep >= 1 {
					continue
				}
			}
		}
		baseOutcome, err := parseOutcomeMap(ev.OutcomeDeltas)
		if err != nil {
			return nil, ChoiceEffect{}, fmt.Errorf("event %s outcome: %w", ev.ID, err)
		}
		choices, err := projectChoices(ev, *s, baseOutcome)
		if err != nil {
			return nil, ChoiceEffect{}, err
		}
		if len(choices) < 2 {
			continue
		}
		if len(choices) > 6 {
			choices = choices[:6]
		}
		weight := rarityWeight(ev.Rarity)
		if weight <= 0 {
			continue
		}
		candidates = append(candidates, candidate{event: ev, choices: choices, weight: weight})
	}
	if len(candidates) == 0 {
		return nil, ChoiceEffect{}, errors.New("no eligible events for current state")
	}
	total := 0
	for _, c := range candidates {
		total += c.weight
	}
	pick := stream.Intn(total)
	acc := 0
	var sel candidate
	for _, c := range candidates {
		acc += c.weight
		if pick < acc {
			sel = c
			break
		}
	}
	ctx := &EventContext{Event: sel.event, Choices: sel.choices}
	for i := range ctx.Choices {
		ctx.Choices[i].Index = i
	}
	effect, err := sel.event.EffectsOnSelect.toChoiceEffect()
	if err != nil {
		return nil, ChoiceEffect{}, fmt.Errorf("event %s effects: %w", sel.event.ID, err)
	}
	return ctx, effect, nil
}

func tierMatches(tier string, preArrival bool, s Survivor) bool {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "pre_arrival":
		return preArrival
	case "post_arrival":
		return !preArrival
	case "researcher":
		return s.Environment.WorldDay < 0
	case "any", "":
		return true
	default:
		return true
	}
}

func eventPreconditionsSatisfied(pre EventPre, s Survivor) bool {
	if rel := strings.ToLower(pre.LADRelation); rel != "" {
		switch rel {
		case "before":
			if !(s.Environment.WorldDay < s.Environment.LAD) {
				return false
			}
		case "arrival":
			if s.Environment.WorldDay != s.Environment.LAD {
				return false
			}
		case "after":
			if !(s.Environment.WorldDay >= s.Environment.LAD) {
				return false
			}
		}
	}
	if len(pre.LocationTypes) > 0 && !containsFold(pre.LocationTypes, string(s.Environment.Location)) {
		return false
	}
	if len(pre.TimeOfDay) > 0 && !containsFold(pre.TimeOfDay, s.Environment.TimeOfDay) {
		return false
	}
	if pre.MinDay != nil && s.Environment.WorldDay < *pre.MinDay {
		return false
	}
	if pre.MaxDay != nil && s.Environment.WorldDay > *pre.MaxDay {
		return false
	}
	if len(pre.ForbidConds) > 0 && survivorHasAnyCondition(s, pre.ForbidConds) {
		return false
	}
	if len(pre.RequireConds) > 0 && !survivorHasAllConditions(s, pre.RequireConds) {
		return false
	}
	if len(pre.SkillsAnyMin) > 0 {
		satisfied := false
		for skillName, min := range pre.SkillsAnyMin {
			lvl := s.Skills[Skill(skillName)]
			if lvl >= min {
				satisfied = true
				break
			}
		}
		if !satisfied {
			return false
		}
	}
	return true
}

func projectChoices(ev Event, s Survivor, base ChoiceOutcome) ([]Choice, error) {
	var out []Choice
	for _, ec := range ev.Choices {
		if !choiceEligible(ec.Gating, s) {
			continue
		}
		arch := ""
		if len(ec.Archetypes) > 0 {
			arch = ec.Archetypes[0]
		}
		risk := RiskLow
		switch strings.ToLower(ec.BaseRisk) {
		case "moderate":
			risk = RiskModerate
		case "high":
			risk = RiskHigh
		}
		timeCost := ec.BaseCost.TimeMin
		if timeCost <= 0 {
			timeCost = 1
		}
		outcome := cloneOutcome(base)
		if len(ec.Outcome) > 0 {
			override, err := parseOutcomeMap(ec.Outcome)
			if err != nil {
				return nil, fmt.Errorf("event %s choice %s outcome: %w", ev.ID, ec.ID, err)
			}
			for k, v := range override {
				outcome[k] = v
			}
		}
		eff, err := ec.Effects.toChoiceEffect()
		if err != nil {
			return nil, fmt.Errorf("event %s choice %s effects: %w", ev.ID, ec.ID, err)
		}
		choice := Choice{
			Index:     0,
			ID:        fmt.Sprintf("%s:%s", ev.ID, ec.ID),
			Label:     ec.Label,
			Archetype: arch,
			Risk:      risk,
			Cost: Cost{
				Time:    timeCost,
				Fatigue: ec.BaseCost.Fatigue,
				Hunger:  ec.BaseCost.Hunger,
				Thirst:  ec.BaseCost.Thirst,
			},
			Outcome:     outcome,
			Effects:     eff,
			SourceEvent: ev.ID,
		}
		if survivorHasCondition(s, ConditionExhaustion) && isHighExertionChoice(choice) {
			continue
		}
		out = append(out, choice)
	}
	return out, nil
}

func choiceEligible(g ChoiceGating, s Survivor) bool {
	if len(g.RequireConditions) > 0 && !survivorHasAllConditions(s, g.RequireConditions) {
		return false
	}
	if len(g.ForbidConditions) > 0 && survivorHasAnyCondition(s, g.ForbidConditions) {
		return false
	}
	if len(g.SkillsMin) > 0 {
		for skillName, min := range g.SkillsMin {
			lvl := s.Skills[Skill(skillName)]
			if lvl < min {
				return false
			}
		}
	}
	return true
}

func survivorHasAnyCondition(s Survivor, conds []string) bool {
	for _, cond := range conds {
		target := Condition(cond)
		for _, existing := range s.Conditions {
			if existing == target {
				return true
			}
		}
	}
	return false
}

func survivorHasAllConditions(s Survivor, conds []string) bool {
	if len(conds) == 0 {
		return true
	}
	for _, cond := range conds {
		target := Condition(cond)
		found := false
		for _, existing := range s.Conditions {
			if existing == target {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func parseOutcomeMap(raw map[string]string) (ChoiceOutcome, error) {
	if len(raw) == 0 {
		return make(ChoiceOutcome), nil
	}
	out := make(ChoiceOutcome, len(raw))
	for key, expr := range raw {
		stat, ok := parseStatKey(key)
		if !ok {
			return nil, fmt.Errorf("unknown stat %s", key)
		}
		rng, err := parseDeltaRange(expr)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", key, err)
		}
		out[stat] = rng
	}
	return out, nil
}

func cloneOutcome(src ChoiceOutcome) ChoiceOutcome {
	if len(src) == 0 {
		return make(ChoiceOutcome)
	}
	out := make(ChoiceOutcome, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func parseStatKey(key string) (StatKey, bool) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case string(StatHealth):
		return StatHealth, true
	case string(StatHunger):
		return StatHunger, true
	case string(StatThirst):
		return StatThirst, true
	case string(StatFatigue):
		return StatFatigue, true
	case string(StatMorale):
		return StatMorale, true
	default:
		return "", false
	}
}

func parseDeltaRange(expr string) (DeltaRange, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return DeltaRange{Min: 0, Max: 0}, nil
	}
	if strings.Contains(trimmed, "..") {
		parts := strings.SplitN(trimmed, "..", 2)
		if len(parts) != 2 {
			return DeltaRange{}, fmt.Errorf("invalid range %s", expr)
		}
		minVal, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return DeltaRange{}, err
		}
		maxVal, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return DeltaRange{}, err
		}
		if minVal > maxVal {
			minVal, maxVal = maxVal, minVal
		}
		return DeltaRange{Min: minVal, Max: maxVal}, nil
	}
	val, err := strconv.Atoi(trimmed)
	if err != nil {
		return DeltaRange{}, err
	}
	return DeltaRange{Min: val, Max: val}, nil
}

func containsFold(list []string, target string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}

// VerifyEventsPack ensures at least one valid event YAML exists in the directory.
func VerifyEventsPack(dir string) error {
	// Quick filesystem check first for clearer error
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("events: cannot read %s: %w", dir, err)
	}
	foundYAML := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".yaml" || ext == ".yml" {
			foundYAML = true
			break
		}
	}
	if !foundYAML {
		return errors.New("events: no *.yaml in assets/events")
	}
	// Load/validate via loader to catch schema problems
	_, err = loadEvents(dir)
	return err
}
