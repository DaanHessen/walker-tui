package engine

import "testing"

func testSurvivorForDay(day int, lad int) Survivor {
    seed, _ := NewRunSeed("test")
    s := NewFirstSurvivor(seed.Stream("s"), "USAMRIID/Fort Detrick (USA)")
    s.Environment.WorldDay = day
    s.Environment.LAD = lad
    s.updateInfectionPresence()
    return s
}

func TestChoiceCountBounds(t *testing.T) {
    seed, _ := NewRunSeed("choice-bounds")
    s := testSurvivorForDay(0, 5) // pre-arrival
    history := EventHistory{Events: map[string]EventState{}, Arcs: map[string]ArcState{}}
    choices, _, err := GenerateChoices(seed.Stream("g"), &s, history, 0)
    if err != nil {
        t.Fatalf("GenerateChoices error: %v", err)
    }
    if n := len(choices); n < 2 || n > 6 {
        t.Fatalf("choices len out of bounds: %d", n)
    }
}

func TestPreArrivalFiltering(t *testing.T) {
    seed, _ := NewRunSeed("pre-arrival")
    s := testSurvivorForDay(0, 5) // pre-arrival
    history := EventHistory{Events: map[string]EventState{}, Arcs: map[string]ArcState{}}
    ctx, _, err := SelectAndBuildChoices(seed.Stream("sel"), &s, choiceConfig{}, history, 0)
    if err != nil {
        t.Fatalf("SelectAndBuildChoices: %v", err)
    }
    // ensure selected event is not explicitly post_arrival
    if ctx.Event.Tier == "post_arrival" {
        t.Fatalf("selected post_arrival event during pre-arrival")
    }
}

