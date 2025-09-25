package engine

import "testing"

// helper to build a simple non-arc event with given rarity
func makeSimpleEvent(id, rarity string) Event {
	return Event{
		ID:     id,
		Name:   id,
		Tier:   "any",
		Rarity: rarity,
		Pre:    EventPre{},
		Choices: []EventChoice{
			{ID: "c1", Label: "Option 1", Archetypes: []string{"act"}, BaseRisk: "low", BaseCost: EventCostSpec{TimeMin: 1}},
			{ID: "c2", Label: "Option 2", Archetypes: []string{"act"}, BaseRisk: "low", BaseCost: EventCostSpec{TimeMin: 1}},
		},
	}
}

func TestRarity_DistributionTendsToWeights(t *testing.T) {
	evCommon := makeSimpleEvent("ev_common", "common")
	evUncommon := makeSimpleEvent("ev_uncommon", "uncommon")
	evRare := makeSimpleEvent("ev_rare", "rare")
	evs := []Event{evCommon, evUncommon, evRare}

	withTestEvents(evs, func() {
		seed, _ := NewRunSeed("rarity-dist")
		w := NewWorld(seed, "1.0.0")
		s := NewFirstSurvivor(seed.Stream("survivor#0"), w.OriginSite)
		counts := map[string]int{}
		total := 2000
		for i := 0; i < total; i++ {
			ctx, _, err := SelectAndBuildChoices(seed.Stream("sel").Child("i").Child(string(rune(i))), &s, choiceConfig{}, EventHistory{}, i)
			if err != nil {
				t.Fatalf("select error: %v", err)
			}
			counts[ctx.Event.ID]++
		}
		c, u, r := counts[evCommon.ID], counts[evUncommon.ID], counts[evRare.ID]
		// Expect ordering: common > uncommon > rare
		if !(c > u && u > r) {
			t.Fatalf("unexpected ordering c=%d u=%d r=%d", c, u, r)
		}
		// Rough ratio check with generous bounds (weights 5:3:1)
		ratioCU := float64(c) / float64(u)
		if ratioCU < 1.3 || ratioCU > 2.5 {
			t.Fatalf("common:uncommon ratio out of bounds: %.2f (c=%d u=%d)", ratioCU, c, u)
		}
		ratioUR := float64(u) / float64(r)
		if ratioUR < 2.0 || ratioUR > 5.0 {
			t.Fatalf("uncommon:rare ratio out of bounds: %.2f (u=%d r=%d)", ratioUR, u, r)
		}
	})
}
