package text

import (
	"strings"
	"testing"

	"github.com/DaanHessen/walker-tui/internal/engine"
)

func TestSceneCacheKeyDeterminism(t *testing.T) {
	state1 := map[string]any{"a": 1, "b": "x"}
	state2 := map[string]any{"a": 1, "b": "x"}
	k1, _ := SceneCacheKey(state1)
	k2, _ := SceneCacheKey(state2)
	if string(k1) != string(k2) {
		t.Fatal("SceneCacheKey not stable for equivalent state")
	}
	state3 := map[string]any{"a": 2, "b": "x"}
	k3, _ := SceneCacheKey(state3)
	if string(k1) == string(k3) {
		t.Fatal("SceneCacheKey identical for different state")
	}
}

func TestOutcomeCacheKeyDeterminism(t *testing.T) {
	state := map[string]any{"day": 1}
	choice := engine.Choice{ID: "c1", Label: "do x"}
	delta := engine.Stats{Fatigue: 1}
	k1, _ := OutcomeCacheKey(state, choice, delta)
	k2, _ := OutcomeCacheKey(state, choice, delta)
	if string(k1) != string(k2) {
		t.Fatal("OutcomeCacheKey not stable")
	}
	k3, _ := OutcomeCacheKey(state, engine.Choice{ID: "c2"}, delta)
	if string(k1) == string(k3) {
		t.Fatal("OutcomeCacheKey should differ for different choice")
	}
}

func TestValidateNarrative_LADGate(t *testing.T) {
	// Build a long-enough text to satisfy word-count validation for scene
	fragment := "You see infected figures nearby. "
	long := strings.TrimSpace(strings.Repeat(fragment, 30)) // ~150 words
	st := map[string]any{"infected_present": false}
	if validateNarrative(long, true, st) {
		t.Fatal("validateNarrative should fail mentioning infected pre-arrival")
	}
	st["infected_present"] = true
	if !validateNarrative(long, true, st) {
		t.Fatal("validateNarrative should allow after arrival")
	}
}
