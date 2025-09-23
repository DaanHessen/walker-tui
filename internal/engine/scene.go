package engine

import (
	"math/rand"
)

// Choice mechanical representation (separate from rendered markdown line)
type Choice struct {
	Index int
	Label string
	Cost  Cost
	Risk  RiskLevel
	Delta Stats // simple stat impact for prototype
}

type Cost struct {
	Time   int // abstract units
	Fatigue int
	Hunger  int
	Thirst  int
}

// GenerateChoices returns a small deterministic set based on RNG.
func GenerateChoices(r *rand.Rand, s Survivor) []Choice {
	choices := []Choice{
		{Index: 0, Label: "Forage nearby", Cost: Cost{Time: 1, Fatigue: 5}, Risk: RiskLow, Delta: Stats{Hunger: -10, Thirst: -5, Fatigue: 5}},
		{Index: 1, Label: "Rest and recover", Cost: Cost{Time: 1}, Risk: RiskLow, Delta: Stats{Fatigue: -10, Morale: 2}},
	}
	// Add a situational third option occasionally
	if r.Intn(100) < 40 {
		choices = append(choices, Choice{Index: len(choices), Label: "Scout street", Cost: Cost{Time: 1, Fatigue: 8}, Risk: RiskModerate, Delta: Stats{Fatigue: 5, Morale: 3}})
	}
	return choices
}

// ApplyChoice applies mechanical deltas and returns resulting stats delta.
func ApplyChoice(s *Survivor, c Choice) Stats {
	d := c.Delta
	// Convert cost to drains
	d.Fatigue += c.Cost.Fatigue
	d.Hunger += c.Cost.Hunger
	d.Thirst += c.Cost.Thirst
	s.UpdateStats(d)
	s.Tick() // baseline progression
	s.EvaluateDeath()
	return d
}
