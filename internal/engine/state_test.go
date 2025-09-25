package engine

import "testing"

func TestFirstSurvivorLADZero(t *testing.T) {
    seed, _ := NewRunSeed("lad-first")
    s := NewFirstSurvivor(seed.Stream("s"), "USAMRIID/Fort Detrick (USA)")
    if s.Environment.LAD != 0 {
        t.Fatalf("expected first survivor LAD=0, got %d", s.Environment.LAD)
    }
    if s.Environment.WorldDay < -9 || s.Environment.WorldDay > 0 {
        t.Fatalf("first survivor world day out of expected range: %d", s.Environment.WorldDay)
    }
}

