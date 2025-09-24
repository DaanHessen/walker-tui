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
	Time    int // abstract units
	Fatigue int
	Hunger  int
	Thirst  int
}

// GenerateChoices returns a small deterministic set based on RNG and settings hints.
func GenerateChoices(r *rand.Rand, s Survivor, opts ...ChoiceOption) []Choice {
	cfg := choiceConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	choices := []Choice{
		{Index: 0, Label: "Forage nearby", Cost: Cost{Time: 1, Fatigue: 5}, Risk: RiskLow, Delta: Stats{Hunger: -10, Thirst: -5, Fatigue: 5}},
		{Index: 1, Label: "Rest and recover", Cost: Cost{Time: 1}, Risk: RiskLow, Delta: Stats{Fatigue: -10, Morale: 2}},
	}
	if r.Intn(100) < 40 { // situational
		choices = append(choices, Choice{Index: len(choices), Label: "Scout street", Cost: Cost{Time: 1, Fatigue: 8}, Risk: RiskModerate, Delta: Stats{Fatigue: 5, Morale: 3}})
	}
	// Infection gating: before infected arrive, downgrade moderate risks (world still quiet)
	if !cfg.infected {
		for i := range choices {
			if choices[i].Risk == RiskModerate {
				choices[i].Risk = RiskLow
			}
		}
	}
	// Scarcity: reduce positive gains and increase costs slightly
	if cfg.scarcity {
		for i := range choices {
			c := &choices[i]
			if c.Delta.Hunger < 0 {
				c.Delta.Hunger = int(float64(c.Delta.Hunger) * 0.6)
			}
			if c.Delta.Thirst < 0 {
				c.Delta.Thirst = int(float64(c.Delta.Thirst) * 0.6)
			}
			if c.Delta.Fatigue < 0 {
				c.Delta.Fatigue = int(float64(c.Delta.Fatigue) * 0.8)
			}
			c.Cost.Fatigue = int(float64(c.Cost.Fatigue)*1.2 + 0.5)
		}
	}
	// Text density could influence number of choices (rich -> add one more low impact option)
	if cfg.textDensity == "rich" && len(choices) < 4 {
		choices = append(choices, Choice{Index: len(choices), Label: "Organize supplies", Cost: Cost{Time: 1}, Risk: RiskLow, Delta: Stats{Morale: 2}})
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

// ChoiceOption functional options to pass runtime settings.
type ChoiceOption func(*choiceConfig)

type choiceConfig struct {
	scarcity    bool
	textDensity string
	infected    bool
}

func WithScarcity(b bool) ChoiceOption        { return func(c *choiceConfig) { c.scarcity = b } }
func WithTextDensity(d string) ChoiceOption   { return func(c *choiceConfig) { c.textDensity = d } }
func WithInfectedPresent(b bool) ChoiceOption { return func(c *choiceConfig) { c.infected = b } }
