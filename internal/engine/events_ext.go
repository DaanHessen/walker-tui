package engine

import (
	"fmt"
	"strings"
	"unicode"
)

type catalogTemplate struct {
	tier      string
	scale     string
	weight    int
	cooldown  int
	once      bool
	scenarios []string
	locations []string
}

func init() {
	expansions := buildExpandedCatalog()
	eventCatalog = append(eventCatalog, expansions...)
}

func buildExpandedCatalog() []EventBlueprint {
	templates := []catalogTemplate{
		{
			tier:     "pre_arrival",
			scale:    "minor",
			weight:   6,
			cooldown: 1,
			scenarios: []string{
				"Pharmacy Stock Squeeze",
				"Civic Center Tension",
				"Transit Hub Bottleneck",
				"Volunteer Briefing Lull",
				"Neighborhood Supply Rumor",
				"Clinic Triage Overflow",
				"Food Queue Anxiety",
				"Water Station Jitters",
				"Grocery Dash",
				"Courier Dispatch Fumble",
				"Shelter Intake Debate",
				"Community Watch Meetup",
				"Emergency Drill Prep",
				"Pharmaceutical Recall Scare",
				"Crowd Control Practice",
			},
			locations: []string{
				"Midtown Pharmacy",
				"South Pier Market",
				"North Loop Grocer",
				"Union Bus Terminal",
				"Civic Theater Plaza",
				"Riverside Pump Station",
				"Old Mill Food Bank",
				"Lakeside Library",
				"Eastside Schoolyard",
				"Hilltop Community Hall",
				"Warehouse Row",
				"Maple Avenue Co-op",
				"Harbor Ferry Landing",
				"Fabric District",
				"Garden Terrace Square",
			},
		},
		{
			tier:     "pre_arrival",
			scale:    "major",
			weight:   3,
			cooldown: 2,
			scenarios: []string{
				"Border Cordons Clash",
				"Hospital Access Protest",
				"Airfield Evacuation Push",
				"Harbor Quarantine Standoff",
				"Government Briefing Meltdown",
				"Rail Depot Shutdown",
				"Mall Evacuation Drill",
				"Media Crew Flashpoint",
				"Search Warrant Stalemate",
				"Stadium Containment Dry Run",
			},
			locations: []string{
				"Westbridge Checkpoint",
				"South Harbor Gate",
				"North Loop Overpass",
				"City Hall Steps",
				"Dry Dock Eight",
				"Runway Echo",
				"Canal Lift Bridge",
				"Logistics Bay Four",
				"Terminal Annex",
				"Courier Tower Lobby",
				"Parliament Rotunda",
				"Central Dispatch Bay",
			},
		},
		{
			tier:     "any",
			scale:    "minor",
			weight:   5,
			cooldown: 1,
			scenarios: []string{
				"Radio Signal Haul",
				"Supply Cache Tip",
				"Evacuee Escort",
				"Cookfire Rotation",
				"Greenhouse Repair",
				"Pipeline Inspection",
				"Utility Reboot",
				"Livestock Scare",
				"Water Quality Sweep",
				"Seed Vault Inventory",
				"Barter Market Gossip",
				"Makeshift Workshop Fix",
				"Satellite Dish Alignment",
				"Long-Range Recon Brief",
				"Quiet Patrol",
			},
			locations: []string{
				"Verdant Rooftop",
				"Canal Lock Station",
				"Signal Relay Barn",
				"Underground Sluice",
				"Skybridge Atrium",
				"Hilltop Observatory",
				"Sunken Plaza",
				"Ferry Causeway",
				"Repurposed Factory Floor",
				"Creekside Cottages",
				"Wind Farm Service Deck",
				"Glasshouse Corridor",
				"Transit Plaza",
				"Bluffside Shelter",
				"Dam Inspection Walk",
			},
		},
		{
			tier:     "any",
			scale:    "major",
			weight:   3,
			cooldown: 2,
			scenarios: []string{
				"Radiation Alarm Investigation",
				"Convoy Coordination Summit",
				"Resource Council Debate",
				"Regional Frequency Summit",
				"Supply Union Arbitration",
				"Disaster Drill Walkthrough",
				"Security Doctrine Review",
				"Logistics Board Crisis",
				"Strategic Alliance Moot",
				"Emergency Law Session",
			},
			locations: []string{
				"Operations Dome",
				"River Delta Command Post",
				"Northern Hangar",
				"Satellite Relay Ridge",
				"Institute War Room",
				"Irrigation Barrage",
				"Skyline Conference Deck",
				"Depot Control Tower",
				"Transit Authority Hub",
				"Outpost Summit Hall",
				"Seaside Parliament",
				"Defense League Rotunda",
			},
		},
		{
			tier:     "post_arrival",
			scale:    "minor",
			weight:   5,
			cooldown: 1,
			scenarios: []string{
				"Scavenger Relay",
				"Trail Bait Extraction",
				"Flooded Basement Sweep",
				"Signal Beacon Jury-rig",
				"Spore Bloom Burn",
				"Barricade Patch",
				"Tunnel Vent Purge",
				"Field Clinic Run",
				"Breach Alarm Reset",
				"Perimeter Sensor Check",
				"Smuggler Cache Probe",
				"Missing Scout Search",
				"Supply Drop Retrieval",
				"Courier Ambush Recovery",
				"Generator Fuel Haul",
			},
			locations: []string{
				"Collapsed Mall",
				"Flooded Underpass",
				"Quarantined Metro",
				"Sunken Parking Deck",
				"Breachline Farmstead",
				"Pylon Watch",
				"Sunset Viaduct",
				"Broken Causeway",
				"Fogged Shipyard",
				"Signal Spire",
				"Derailed Freight Yard",
				"Riverfront Stronghold",
				"Chimney Stack Works",
				"Beacon Ruins",
				"Moonlit Quarry",
			},
		},
		{
			tier:     "post_arrival",
			scale:    "major",
			weight:   2,
			cooldown: 3,
			scenarios: []string{
				"Horde Spillover",
				"Mutated Pack Hunt",
				"Refugee Stronghold Siege",
				"Breakout Containment",
				"River Barricade Collapse",
				"Tower Evacuation Spiral",
				"Sanctuary Coup",
				"Nightfall Beacon Failure",
				"Warband Ambush",
				"Quarantine Ring Breach",
			},
			locations: []string{
				"Crimson Ferry Port",
				"Overgrown Campus",
				"Verdigris Plaza",
				"Sunken Cathedral",
				"Hazard Research Annex",
				"Canopy Flight Deck",
				"Mirelock Factory",
				"Feral Orchard",
				"Signal Ridge",
				"Silent Harbor",
				"Citadel Airlock",
				"Shattered Stadium",
			},
		},
		{
			tier:     "post_arrival",
			scale:    "major",
			weight:   1,
			cooldown: 5,
			once:     true,
			scenarios: []string{
				"Last Convoy Stand",
				"Deep Vault Awakening",
				"Harbor Exodus Finale",
				"Grid Collapse Spiral",
				"Bio-Lab Evacuation Gamble",
				"Skyport Final Sortie",
				"Cryo Bunker Override",
				"River Gate Cataclysm",
			},
			locations: []string{
				"Obsidian Breakwater",
				"Atlas Reactor",
				"Sanctum Bunker",
				"Ironveil Bridge",
				"Hawthorne Deck",
				"Sable Arcology",
				"Neon Trench",
				"Citadel Plaza",
			},
		},
	}

	existing := make(map[string]struct{}, len(eventCatalog))
	for _, ev := range eventCatalog {
		existing[ev.ID] = struct{}{}
	}

	var expanded []EventBlueprint
	for _, tmpl := range templates {
		for _, scenario := range tmpl.scenarios {
			for _, location := range tmpl.locations {
				name := fmt.Sprintf("%s at %s", scenario, location)
				id := slugify(strings.Join([]string{scenario, location, tmpl.tier}, " "))
				if _, ok := existing[id]; ok {
					continue
				}
				expanded = append(expanded, EventBlueprint{
					ID:             id,
					Name:           name,
					Tier:           tmpl.tier,
					Scale:          tmpl.scale,
					Weight:         tmpl.weight,
					CooldownScenes: tmpl.cooldown,
					OncePerRun:     tmpl.once,
				})
				existing[id] = struct{}{}
			}
		}
	}
	return expanded
}

func slugify(raw string) string {
	raw = strings.ToLower(raw)
	var b strings.Builder
	underscore := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if underscore {
				b.WriteRune('_')
				underscore = false
			}
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == ':' || r == '/' || r == '\\':
			underscore = true
		case r == '&':
			if underscore {
				b.WriteRune('_')
				underscore = false
			}
			b.WriteString("and")
		case r == '+':
			if underscore {
				b.WriteRune('_')
				underscore = false
			}
			b.WriteString("plus")
		default:
			underscore = true
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		return "event"
	}
	return slug
}
