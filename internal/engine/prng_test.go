package engine

import "testing"

func TestRunSeedDeterminism(t *testing.T) {
    r1, _ := NewRunSeed("alpha-seed")
    r2, _ := NewRunSeed("alpha-seed")
    s1 := r1.Stream("x").Intn(1000000)
    s2 := r2.Stream("x").Intn(1000000)
    if s1 != s2 {
        t.Fatalf("streams differ: %d vs %d", s1, s2)
    }
    // child streams
    c1 := r1.Stream("x").Child("y").Intn(1000000)
    c2 := r2.Stream("x").Child("y").Intn(1000000)
    if c1 != c2 {
        t.Fatalf("child streams differ: %d vs %d", c1, c2)
    }
}

