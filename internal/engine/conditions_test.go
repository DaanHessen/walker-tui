package engine

import "testing"

func newTestSurvivor() *Survivor {
    seed, _ := NewRunSeed("cond-seed")
    s := NewFirstSurvivor(seed.Stream("s"), "USAMRIID/Fort Detrick (USA)")
    return &s
}

func TestDehydrationTriggerAndRemoval(t *testing.T) {
    s := newTestSurvivor()
    s.Stats.Thirst = 85
    // three scenes with high thirst to trigger
    for i := 0; i < 3; i++ {
        advanceConditions(s, DifficultyStandard, Choice{}, Stats{})
    }
    if !survivorHasCondition(*s, ConditionDehydration) {
        t.Fatalf("expected dehydration after 3 thirsty scenes")
    }
    // strong rehydration over two scenes; ensure threshold is met when recovery reaches 2
    advanceConditions(s, DifficultyStandard, Choice{}, Stats{Thirst: -12})
    s.Stats.Thirst = 40 // set before second increment
    advanceConditions(s, DifficultyStandard, Choice{}, Stats{Thirst: -12})
    if survivorHasCondition(*s, ConditionDehydration) {
        t.Fatalf("expected dehydration removed after recovery")
    }
}
