package engine

import "testing"

// helper to build a simple arc event
func makeArcEvent(id string, step int, delay int, next []string) Event {
	return Event{
		ID:     id,
		Name:   id,
		Tier:   "any",
		Rarity: "common",
		Pre:    EventPre{},
		Arc: &EventArc{
			ID:                 "arcA",
			Step:               step,
			NextMinDelayScenes: delay,
			NextCandidates:     next,
		},
		Choices: []EventChoice{
			{ID: "c1", Label: "Do it", Archetypes: []string{"act"}, BaseRisk: "low", BaseCost: EventCostSpec{TimeMin: 1}},
			{ID: "c2", Label: "Or not", Archetypes: []string{"act"}, BaseRisk: "low", BaseCost: EventCostSpec{TimeMin: 1}},
		},
	}
}

func withTestEvents(evs []Event, fn func()) {
	prev := testEventsOverride
	testEventsOverride = func() ([]Event, bool) { return evs, true }
	defer func() { testEventsOverride = prev }()
	fn()
}

func TestArcSequencing_StepAndDelayAndCandidates(t *testing.T) {
	ev1 := makeArcEvent("arc_ev_1", 1, 0, []string{"arc_ev_2"})
	ev2 := makeArcEvent("arc_ev_2", 2, 2, []string{"arc_ev_3"})
	ev3 := makeArcEvent("arc_ev_3", 3, 0, nil)
	evs := []Event{ev1, ev2, ev3}

	withTestEvents(evs, func() {
		seed, _ := NewRunSeed("arc-seq")
		w := NewWorld(seed, "1.0.0")
		s := NewFirstSurvivor(seed.Stream("survivor#0"), w.OriginSite)

		// Step 1 should be chosen first (others gated)
		ctx1, _, err := SelectAndBuildChoices(seed.Stream("sel1"), &s, choiceConfig{}, EventHistory{}, 0)
		if err != nil {
			t.Fatalf("sel1 error: %v", err)
		}
		if ctx1.Event.ID != ev1.ID {
			t.Fatalf("expected %s first, got %s", ev1.ID, ctx1.Event.ID)
		}
		// history after firing step1 at scene 0
		hist := EventHistory{Events: map[string]EventState{ev1.ID: {LastSceneIdx: 0, OnceFired: true}}, Arcs: map[string]ArcState{"arcA": {LastStep: 1, LastSceneIdx: 0, LastEventID: ev1.ID}}}

		// At scene 1, step2 should be blocked by min delay 2
		_, _, err = SelectAndBuildChoices(seed.Stream("sel2"), &s, choiceConfig{}, hist, 1)
		if err == nil {
			t.Fatalf("expected no eligible events at scene 1 due to delay")
		}

		// At scene 2, step2 becomes eligible and should be selected (candidates restrict)
		ctx2, _, err := SelectAndBuildChoices(seed.Stream("sel3"), &s, choiceConfig{}, hist, 2)
		if err != nil {
			t.Fatalf("sel3 error: %v", err)
		}
		if ctx2.Event.ID != ev2.ID {
			t.Fatalf("expected %s second, got %s", ev2.ID, ctx2.Event.ID)
		}

		// Update history to after step2 at scene 2
		hist2 := EventHistory{Events: map[string]EventState{ev2.ID: {LastSceneIdx: 2, OnceFired: true}}, Arcs: map[string]ArcState{"arcA": {LastStep: 2, LastSceneIdx: 2, LastEventID: ev2.ID}}}
		ctx3, _, err := SelectAndBuildChoices(seed.Stream("sel4"), &s, choiceConfig{}, hist2, 3)
		if err != nil {
			t.Fatalf("sel4 error: %v", err)
		}
		if ctx3.Event.ID != ev3.ID {
			t.Fatalf("expected %s third, got %s", ev3.ID, ctx3.Event.ID)
		}
	})
}

func TestRarityWeight_Simple(t *testing.T) {
	if rarityWeight("rare") >= rarityWeight("common") {
		t.Fatalf("expected rare < common weight")
	}
	if rarityWeight("uncommon") <= rarityWeight("rare") {
		t.Fatalf("expected uncommon > rare weight")
	}
}
