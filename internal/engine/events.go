package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// DirectorPlanner represents an AI director capable of selecting an event and emitting choice scaffolding.
type DirectorPlanner interface {
	PlanEvent(ctx context.Context, req DirectorRequest) (DirectorPlan, error)
}

// DirectorRequest packages survivor context and currently eligible events for the planner.
type DirectorRequest struct {
	State         map[string]any   `json:"state"`
	Available     []EventBlueprint `json:"available_events"`
	History       HistorySnapshot  `json:"history"`
	SceneIndex    int              `json:"scene_index"`
	Scarcity      bool             `json:"scarcity"`
	TextDensity   string           `json:"text_density"`
	Difficulty    Difficulty       `json:"difficulty"`
	InfectedLocal bool             `json:"infected_local"`
}

// HistorySnapshot conveys recent director decisions to help avoid repetition.
type HistorySnapshot struct {
	LastEvent string   `json:"last_event"`
	Recent    []string `json:"recent"`
}

// DirectorPlan is the planner's response describing the next event and its choices.
type DirectorPlan struct {
	EventID   string
	EventName string
	Guidance  string
	Choices   []PlannedChoice
}

// PlannedChoice captures a single proposed choice in the director plan.
type PlannedChoice struct {
	Label     string
	Archetype string
	Cost      PlanCost
	Risk      string
}

// PlanCost mirrors Choice cost inputs provided by the director.
type PlanCost struct {
	Time    int
	Fatigue int
	Hunger  int
	Thirst  int
}

// EventBlueprint holds local metadata for an event (no narrative text).
type EventBlueprint struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Tier           string `json:"tier"` // pre_arrival | post_arrival | any
	Scale          string `json:"scale"`
	Weight         int    `json:"weight"`
	CooldownScenes int    `json:"cooldown_scenes"`
	OncePerRun     bool   `json:"once_per_run"`
}

// EventContext tracks the executed event blueprint plus planner guidance.
type EventContext struct {
	Event    EventBlueprint
	Guidance string
	Choices  []Choice
}

type EventState struct {
	LastSceneIdx       int
	CooldownUntilScene int
	OnceFired          bool
}

type EventHistory struct {
	Events map[string]EventState
	Recent []string // most recent first
}

func (h EventHistory) eventState(id string) EventState {
	if h.Events == nil {
		return EventState{}
	}
	return h.Events[id]
}

func (h EventHistory) snapshot(limit int) HistorySnapshot {
	if limit <= 0 {
		limit = 5
	}
	snap := HistorySnapshot{}
	if len(h.Recent) == 0 {
		return snap
	}
	end := limit
	if end > len(h.Recent) {
		end = len(h.Recent)
	}
	snap.Recent = append([]string{}, h.Recent[:end]...)
	snap.LastEvent = snap.Recent[0]
	return snap
}

// Catalog of available event blueprints.
var eventCatalog = []EventBlueprint{
	{ID: "urban_supply_scramble", Name: "Urban Supply Scramble", Tier: "pre_arrival", Scale: "minor", Weight: 6, CooldownScenes: 1},
	{ID: "checkpoint_tension", Name: "Checkpoint Tension", Tier: "pre_arrival", Scale: "major", Weight: 3, CooldownScenes: 2},
	{ID: "rolling_blackout", Name: "Rolling Blackout", Tier: "pre_arrival", Scale: "minor", Weight: 4, CooldownScenes: 2},
	{ID: "crowd_panic", Name: "Crowd Panic Surge", Tier: "pre_arrival", Scale: "major", Weight: 2, CooldownScenes: 3},
	{ID: "quiet_hour", Name: "Uneasy Quiet Hour", Tier: "any", Scale: "minor", Weight: 6, CooldownScenes: 1},
	{ID: "radio_distress", Name: "Radio Distress Call", Tier: "any", Scale: "minor", Weight: 4, CooldownScenes: 2},
	{ID: "supply_convoy", Name: "Supply Convoy Sighting", Tier: "any", Scale: "minor", Weight: 4, CooldownScenes: 2},
	{ID: "makeshift_clinic", Name: "Makeshift Clinic", Tier: "any", Scale: "minor", Weight: 3, CooldownScenes: 2},
	{ID: "rooftop_signal", Name: "Rooftop Signal", Tier: "post_arrival", Scale: "minor", Weight: 3, CooldownScenes: 2},
	{ID: "neighborhood_breach", Name: "Neighborhood Breach", Tier: "post_arrival", Scale: "major", Weight: 2, CooldownScenes: 3},
	{ID: "street_hunt", Name: "Street Hunt", Tier: "post_arrival", Scale: "major", Weight: 2, CooldownScenes: 3},
	{ID: "hospital_overrun", Name: "Hospital Overrun", Tier: "post_arrival", Scale: "major", Weight: 1, CooldownScenes: 4, OncePerRun: true},
	{ID: "abandoned_lab", Name: "Abandoned Lab Floor", Tier: "post_arrival", Scale: "major", Weight: 1, CooldownScenes: 4, OncePerRun: true},
	{ID: "shelter_dynamics", Name: "Shelter Dynamics", Tier: "any", Scale: "minor", Weight: 5, CooldownScenes: 1},
}

func catalogByID() map[string]EventBlueprint {
	out := make(map[string]EventBlueprint, len(eventCatalog))
	for _, ev := range eventCatalog {
		out[ev.ID] = ev
	}
	return out
}

func availableEventBlueprints(s *Survivor, history EventHistory, sceneIdx int) []EventBlueprint {
	preArrival := s.Environment.WorldDay < s.Environment.LAD
	catalog := catalogByID()
	keys := make([]string, 0, len(catalog))
	for id := range catalog {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	var out []EventBlueprint
	for _, id := range keys {
		bp := catalog[id]
		if bp.OncePerRun {
			state := history.eventState(bp.ID)
			if state.OnceFired {
				continue
			}
		}
		state := history.eventState(bp.ID)
		if state.CooldownUntilScene > sceneIdx {
			continue
		}
		switch strings.ToLower(bp.Tier) {
		case "pre_arrival":
			if !preArrival {
				continue
			}
		case "post_arrival":
			if preArrival {
				continue
			}
		}
		out = append(out, bp)
	}
	return out
}

// GenerateChoices asks the director for an event plan and converts it into mechanical choices.
func GenerateChoices(ctx context.Context, planner DirectorPlanner, stream *Stream, s *Survivor, history EventHistory, sceneIdx int, opts ...ChoiceOption) ([]Choice, *EventContext, error) {
	if planner == nil {
		return nil, nil, errors.New("planner is nil")
	}
	cfg := choiceConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	available := availableEventBlueprints(s, history, sceneIdx)
	if len(available) == 0 {
		return nil, nil, errors.New("no eligible events for current state")
	}
	req := DirectorRequest{
		State:         s.NarrativeState(),
		Available:     available,
		History:       history.snapshot(5),
		SceneIndex:    sceneIdx,
		Scarcity:      cfg.scarcity,
		TextDensity:   cfg.textDensity,
		Difficulty:    cfg.difficulty,
		InfectedLocal: s.Environment.Infected,
	}
	plan, err := planner.PlanEvent(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	catalog := catalogByID()
	bp, ok := catalog[plan.EventID]
	if !ok {
		// allow fallback to name match when ID omitted
		for _, candidate := range catalog {
			if strings.EqualFold(candidate.Name, plan.EventName) {
				bp = candidate
				ok = true
				break
			}
		}
	}
	if !ok {
		return nil, nil, fmt.Errorf("planner selected unknown event %q", plan.EventID)
	}
	state := history.eventState(bp.ID)
	if bp.OncePerRun && state.OnceFired {
		return nil, nil, fmt.Errorf("planner selected once-per-run event %q already used", bp.ID)
	}
	if state.CooldownUntilScene > sceneIdx {
		return nil, nil, fmt.Errorf("planner selected event %q still on cooldown", bp.ID)
	}
	if len(plan.Choices) < 2 || len(plan.Choices) > 6 {
		return nil, nil, fmt.Errorf("planner returned %d choices (must be 2-6)", len(plan.Choices))
	}
	choices := make([]Choice, 0, len(plan.Choices))
	for i, pc := range plan.Choices {
		choice, err := buildChoiceFromPlan(bp.ID, i, pc)
		if err != nil {
			return nil, nil, fmt.Errorf("choice %d invalid: %w", i, err)
		}
		adjustRisk(&choice, *s, cfg)
		choices = append(choices, choice)
	}
	ctxOut := &EventContext{
		Event:    bp,
		Guidance: plan.Guidance,
		Choices:  choices,
	}
	return choices, ctxOut, nil
}

type archetypeProfile struct {
	BaseOutcome ChoiceOutcome
	BaseEffects ChoiceEffect
	BaseCost    Cost
}

var archetypeProfiles = map[string]archetypeProfile{
	"rest": {
		BaseOutcome: ChoiceOutcome{
			StatFatigue: {Min: -12, Max: -8},
			StatMorale:  {Min: 1, Max: 2},
		},
		BaseCost: Cost{Time: 1},
	},
	"forage": {
		BaseOutcome: ChoiceOutcome{
			StatHunger:  {Min: -8, Max: -5},
			StatThirst:  {Min: -6, Max: -3},
			StatFatigue: {Min: 4, Max: 7},
		},
		BaseCost: Cost{Time: 1, Fatigue: 3},
	},
	"scout": {
		BaseOutcome: ChoiceOutcome{
			StatFatigue: {Min: 5, Max: 8},
			StatMorale:  {Min: 1, Max: 2},
		},
		BaseCost: Cost{Time: 1, Fatigue: 4},
	},
	"organize": {
		BaseOutcome: ChoiceOutcome{
			StatMorale: {Min: 2, Max: 4},
		},
		BaseCost: Cost{Time: 1},
	},
	"barricade": {
		BaseOutcome: ChoiceOutcome{
			StatFatigue: {Min: 6, Max: 9},
		},
		BaseCost: Cost{Time: 1, Fatigue: 5},
	},
	"craft": {
		BaseOutcome: ChoiceOutcome{
			StatFatigue: {Min: 4, Max: 7},
			StatMorale:  {Min: 1, Max: 2},
		},
		BaseCost: Cost{Time: 1, Fatigue: 4},
	},
	"diplomacy": {
		BaseOutcome: ChoiceOutcome{
			StatMorale: {Min: 2, Max: 4},
		},
		BaseCost: Cost{Time: 1},
	},
	"observe": {
		BaseOutcome: ChoiceOutcome{
			StatFatigue: {Min: 3, Max: 5},
			StatMorale:  {Min: 1, Max: 1},
		},
		BaseCost: Cost{Time: 1, Fatigue: 2},
	},
}

func buildChoiceFromPlan(eventID string, idx int, pc PlannedChoice) (Choice, error) {
	label := strings.TrimSpace(pc.Label)
	if label == "" {
		return Choice{}, errors.New("label empty")
	}
	archetype := strings.ToLower(strings.TrimSpace(pc.Archetype))
	profile, ok := archetypeProfiles[archetype]
	if !ok {
		return Choice{}, fmt.Errorf("unknown archetype %q", pc.Archetype)
	}
	risk, err := riskFromString(pc.Risk)
	if err != nil {
		return Choice{}, err
	}
	cost := profile.BaseCost
	cost.Time = clampMin(pc.Cost.Time, 1, cost.Time)
	cost.Fatigue = clampWithDefault(pc.Cost.Fatigue, cost.Fatigue)
	cost.Hunger = clampWithDefault(pc.Cost.Hunger, cost.Hunger)
	cost.Thirst = clampWithDefault(pc.Cost.Thirst, cost.Thirst)
	outcome := cloneOutcome(profile.BaseOutcome)
	choice := Choice{
		Index:       idx,
		ID:          fmt.Sprintf("%s:%d", eventID, idx),
		Label:       label,
		Cost:        cost,
		Risk:        risk,
		Archetype:   archetype,
		Outcome:     outcome,
		Effects:     profile.BaseEffects,
		SourceEvent: eventID,
	}
	return choice, nil
}

func riskFromString(raw string) (RiskLevel, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "low", "risk_low", "r1", "":
		return RiskLow, nil
	case "moderate", "medium", "risk_moderate", "r2":
		return RiskModerate, nil
	case "high", "risk_high", "r3":
		return RiskHigh, nil
	default:
		return "", fmt.Errorf("unknown risk level %q", raw)
	}
}

func clampMin(value, min int, fallback int) int {
	if value <= 0 {
		if fallback < min {
			return min
		}
		return fallback
	}
	if value < min {
		return min
	}
	return value
}

func clampWithDefault(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
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

// AllowedArchetypes returns the ordered list of archetypes supported by planner choices.
func AllowedArchetypes() []string {
	keys := make([]string, 0, len(archetypeProfiles))
	for k := range archetypeProfiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
