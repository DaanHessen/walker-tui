package ui

import (
	"testing"

	"github.com/DaanHessen/walker-tui/internal/engine"
)

func TestWorldUsesMixedSeedAfterPersist(t *testing.T) {
	seed, _ := engine.NewRunSeed("abc")
	w := engine.NewWorld(seed, "1.0.0")
	m := model{world: w, runSeed: seed}
	// simulate persist
	m.runSeed = m.runSeed.WithRunContext("run-xyz", "1.0.0")
	m.world.Seed = m.runSeed
	// verify choiceStream derives from mixed seed root by comparing to a fresh world with same op
	mix := seed.WithRunContext("run-xyz", "1.0.0")
	want := mix.Stream("day:0:turn:0:choices").Uint64()
	got := m.choiceStream("choices").Uint64()
	if got != want {
		t.Fatalf("choiceStream not using mixed seed: got %d want %d", got, want)
	}
}
