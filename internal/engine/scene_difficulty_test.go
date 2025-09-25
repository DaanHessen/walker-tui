package engine

import "testing"

func TestDifficultyBaselineDrains(t *testing.T) {
    s := Survivor{Stats: Stats{Health: 100, Hunger: 0, Thirst: 0, Fatigue: 0, Morale: 50}, Meters: baselineMeters()}
    choice := Choice{ID: "t", Cost: Cost{}, Outcome: ChoiceOutcome{}}
    // easy
    s1 := s
    res1 := ApplyChoice(&s1, choice, DifficultyEasy, 0, nil)
    if res1.Delta.Hunger != 1 || res1.Delta.Thirst != 2 || res1.Delta.Fatigue != 1 {
        t.Fatalf("easy drains unexpected: %+v", res1.Delta)
    }
    // standard
    s2 := s
    res2 := ApplyChoice(&s2, choice, DifficultyStandard, 0, nil)
    if res2.Delta.Hunger != 2 || res2.Delta.Thirst != 3 || res2.Delta.Fatigue != 2 {
        t.Fatalf("standard drains unexpected: %+v", res2.Delta)
    }
    // hard
    s3 := s
    res3 := ApplyChoice(&s3, choice, DifficultyHard, 0, nil)
    if res3.Delta.Hunger != 3 || res3.Delta.Thirst != 4 || res3.Delta.Fatigue != 3 {
        t.Fatalf("hard drains unexpected: %+v", res3.Delta)
    }
}
