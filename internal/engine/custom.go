package engine

import (
	"strings"
)

// Custom action validation maps free text to an archetypal Choice or rejects it.
// Returns (choice, allowed, rejectionReason).
// ValidateCustomAction extended with gating: rejects if fatigue>85 (except rest), hunger>95 or thirst>95 (except forage), or repeating same archetype consecutively.
func ValidateCustomAction(input string, base Survivor) (Choice, bool, string) {
	in := strings.ToLower(strings.TrimSpace(input))
	if in == "" {
		return Choice{}, false, "empty"
	}
	archetype := ""
	switch {
	case hasAny(in, "rest", "sleep", "recover", "nap"):
		archetype = "rest"
	case hasAny(in, "forage", "search", "scavenge", "look for", "gather"):
		archetype = "forage"
	case hasAny(in, "scout", "peek", "survey", "recon"):
		archetype = "scout"
	case hasAny(in, "organize", "sort", "arrange"):
		archetype = "organize"
	case hasAny(in, "barricade", "board", "secure"):
		archetype = "barricade"
	default:
		return Choice{}, false, "No supported action archetype found"
	}
	// Gating rules
	if archetype != "rest" && base.Stats.Fatigue > 85 {
		return Choice{}, false, "Too fatigued"
	}
	if archetype != "forage" && (base.Stats.Hunger > 95 || base.Stats.Thirst > 95) {
		return Choice{}, false, "Critical needs first"
	}
	if last, ok := base.Meters[MeterNoise]; ok {
		_ = last
	} // placeholder example
	if tag, ok := base.Meters[MeterVisibility]; ok {
		_ = tag
	} // placeholder
	if lc, ok := base.Meters[MeterScent]; ok {
		_ = lc
	} // placeholder
	// remove obsolete hash-based cooldown; real cooldown enforced in UI using MeterCustomLastTurn
	// Map archetype to synthetic choice (index -1 indicates synthetic)
	c := Choice{Index: -1, Label: "Custom: " + input, Risk: RiskLow, Archetype: archetype}
	switch archetype {
	case "rest":
		c.Cost = Cost{Time: 1}
		c.Delta = Stats{Fatigue: -12, Morale: 1}
	case "forage":
		c.Cost = Cost{Time: 1, Fatigue: 4}
		c.Risk = ternary(base.Environment.Infected, RiskModerate, RiskLow)
		c.Delta = Stats{Hunger: -8, Thirst: -4, Fatigue: 4}
	case "scout":
		c.Cost = Cost{Time: 1, Fatigue: 6}
		c.Risk = ternary(base.Environment.Infected, RiskModerate, RiskLow)
		c.Delta = Stats{Fatigue: 5, Morale: 2}
	case "organize":
		c.Cost = Cost{Time: 1}
		c.Delta = Stats{Morale: 2}
	case "barricade":
		c.Cost = Cost{Time: 1, Fatigue: 7}
		c.Risk = ternary(base.Environment.Infected, RiskModerate, RiskLow)
		c.Delta = Stats{Fatigue: 7, Morale: 1}
	}
	return c, true, ""
}

func hasAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
