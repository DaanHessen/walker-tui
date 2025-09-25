package engine

import (
	"context"
	"testing"
)

type stubPlanner struct {
	plan    DirectorPlan
	err     error
	calls   int
	lastReq *DirectorRequest
}

func (s *stubPlanner) PlanEvent(ctx context.Context, req DirectorRequest) (DirectorPlan, error) {
	s.calls++
	s.lastReq = &req
	if s.err != nil {
		return DirectorPlan{}, s.err
	}
	return s.plan, nil
}

func TestAvailableEventBlueprints_PreArrivalFilters(t *testing.T) {
	seed, _ := NewRunSeed("pre-arrival-filter")
	survivor := NewFirstSurvivor(seed.Stream("survivor"), "USAMRIID/Fort Detrick (USA)")
	survivor.Environment.WorldDay = 0
	survivor.Environment.LAD = 5
	survivor.updateInfectionPresence()

	history := EventHistory{
		Events: map[string]EventState{
			"hospital_overrun":   {OnceFired: true},
			"checkpoint_tension": {CooldownUntilScene: 4},
		},
	}

	events := availableEventBlueprints(&survivor, history, 2)
	if len(events) == 0 {
		t.Fatalf("expected events to be available")
	}

	for _, ev := range events {
		if ev.Tier == "post_arrival" {
			t.Fatalf("post-arrival event %q should be filtered before LAD", ev.ID)
		}
		if ev.ID == "hospital_overrun" {
			t.Fatalf("once-per-run event should be filtered after firing")
		}
		if ev.ID == "checkpoint_tension" {
			t.Fatalf("event on cooldown should be filtered out")
		}
	}
}

func TestGenerateChoices_UsesPlannerPlan(t *testing.T) {
	seed, _ := NewRunSeed("planner-success")
	survivor := NewFirstSurvivor(seed.Stream("sv"), "USAMRIID/Fort Detrick (USA)")
	survivor.Environment.WorldDay = 5
	survivor.Environment.LAD = 5
	survivor.updateInfectionPresence()
	survivor.Skills[SkillScavenging] = 5

	planner := &stubPlanner{
		plan: DirectorPlan{
			EventID:   "checkpoint_tension",
			EventName: "Checkpoint Tension",
			Guidance:  "Keep the crowd under control",
			Choices: []PlannedChoice{
				{
					Label:     "Negotiate with calm authority",
					Archetype: "diplomacy",
					Cost:      PlanCost{Time: 1},
					Risk:      "low",
				},
				{
					Label:     "Search unattended crates",
					Archetype: "forage",
					Cost:      PlanCost{Time: 2, Fatigue: 3},
					Risk:      "moderate",
				},
			},
		},
	}

	history := EventHistory{}
	choices, ctx, err := GenerateChoices(context.Background(), planner, seed.Stream("choices"), &survivor, history, 3)
	if err != nil {
		t.Fatalf("GenerateChoices returned error: %v", err)
	}
	if planner.calls != 1 {
		t.Fatalf("expected planner to be invoked once, got %d", planner.calls)
	}
	if planner.lastReq == nil {
		t.Fatalf("expected planner request to be captured")
	}
	if len(choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(choices))
	}
	if ctx == nil || ctx.Event.ID != "checkpoint_tension" {
		t.Fatalf("unexpected event context: %+v", ctx)
	}
	if choices[0].Label != "Negotiate with calm authority" {
		t.Fatalf("choice label mismatch: %+v", choices[0])
	}
	if choices[1].Archetype != "forage" {
		t.Fatalf("expected forage archetype, got %q", choices[1].Archetype)
	}
	if choices[1].Risk != RiskLow {
		t.Fatalf("expected skill-adjusted risk to be low, got %s", choices[1].Risk)
	}
}

func TestGenerateChoicesRejectsUnknownEvent(t *testing.T) {
	seed, _ := NewRunSeed("planner-error")
	survivor := NewFirstSurvivor(seed.Stream("sv"), "USAMRIID/Fort Detrick (USA)")
	planner := &stubPlanner{
		plan: DirectorPlan{
			EventID: "made_up_event",
			Choices: []PlannedChoice{{
				Label:     "Do something",
				Archetype: "rest",
				Cost:      PlanCost{Time: 1},
				Risk:      "low",
			}, {
				Label:     "Do something else",
				Archetype: "forage",
				Cost:      PlanCost{Time: 1},
				Risk:      "moderate",
			}},
		},
	}
	history := EventHistory{}
	_, _, err := GenerateChoices(context.Background(), planner, seed.Stream("err"), &survivor, history, 0)
	if err == nil {
		t.Fatalf("expected error when planner selects unknown event")
	}
}

func TestEventHistorySnapshot(t *testing.T) {
	hist := EventHistory{Recent: []string{"ev1", "ev2", "ev3"}}
	snap := hist.snapshot(2)
	if len(snap.Recent) != 2 {
		t.Fatalf("expected 2 recent events, got %d", len(snap.Recent))
	}
	if snap.LastEvent != "ev1" {
		t.Fatalf("expected last event ev1, got %s", snap.LastEvent)
	}
}

func TestAllowedArchetypesSorted(t *testing.T) {
	names := AllowedArchetypes()
	if len(names) == 0 {
		t.Fatal("expected archetypes to be defined")
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("expected archetypes sorted, got %v", names)
		}
	}
}
