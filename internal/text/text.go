package text

import (
	"context"
	"fmt"
	"strings"

	"github.com/DaanHessen/walker-tui/internal/engine"
)

// Narrator is the interface used by the game to render prose.
type Narrator interface {
	Scene(ctx context.Context, st any) (string, error)
	Outcome(ctx context.Context, st any, ch any, up any) (string, error)
}

// templateNarrator is a deterministic, offline narrator used as fallback.
type templateNarrator struct{}

func NewTemplateNarrator(seed int64) Narrator { return &templateNarrator{} }

func (t *templateNarrator) Scene(ctx context.Context, st any) (string, error) {
	m, ok := st.(map[string]any)
	if !ok {
		return "[invalid scene state]", nil
	}
	// Basic markdown scaffold matching required ordering subset.
	var b strings.Builder
	b.WriteString("## CHARACTER OVERVIEW\n")
	b.WriteString(fmt.Sprintf("Name: %v | Day: %v\n\n", m["name"], m["world_day"]))
	b.WriteString("## SKILLS\n")
	if skills, ok := m["skills"].(map[engine.Skill]int); ok {
		for k, v := range skills { b.WriteString(fmt.Sprintf("- %s: %d\n", k, v)) }
	}
	b.WriteString("\n## STATS\n")
	if stats, ok := m["stats"].(engine.Stats); ok {
		b.WriteString(fmt.Sprintf("Health %d Hunger %d Thirst %d Fatigue %d Morale %d\n", stats.Health, stats.Hunger, stats.Thirst, stats.Fatigue, stats.Morale))
	}
	b.WriteString("\n## SCENE\n")
	infected := false
	if v, ok := m["infected_present"].(bool); ok { infected = v }
	if infected { b.WriteString("Distant shapes move with unnatural jerks at the edge of town. ") } else { b.WriteString("The streets remain eerily untouched by the outbreak's signs. ") }
	b.WriteString("(Placeholder narrative)\n")
	b.WriteString("\n## CHOICES\n")
	b.WriteString("1. Forage (Cost: time+fatigue, Risk: Low)\n2. Rest (Cost: time, Risk: Low)\n")
	return b.String(), nil
}
func (t *templateNarrator) Outcome(ctx context.Context, st any, ch any, up any) (string, error) {
	return "Outcome: placeholder markdown.", nil
}

// deepSeekNarrator is a placeholder for the online AI narrator.
type deepSeekNarrator struct{}

func NewDeepSeekNarrator(apiKey string) (Narrator, error) {
	// Placeholder: return a template narrator to keep the build local-only for now.
	return &templateNarrator{}, nil
}

// WithFallback returns a narrator that prefers primary and falls back to backup on error.
func WithFallback(primary, fallback Narrator) Narrator { return &fallbackNarrator{p: primary, f: fallback} }

type fallbackNarrator struct{ p, f Narrator }

func (n *fallbackNarrator) Scene(ctx context.Context, st any) (string, error) {
	if n.p == nil { return n.f.Scene(ctx, st) }
	if s, err := n.p.Scene(ctx, st); err == nil { return s, nil }
	return n.f.Scene(ctx, st)
}
func (n *fallbackNarrator) Outcome(ctx context.Context, st any, ch any, up any) (string, error) {
	if n.p == nil { return n.f.Outcome(ctx, st, ch, up) }
	if s, err := n.p.Outcome(ctx, st, ch, up); err == nil { return s, nil }
	return n.f.Outcome(ctx, st, ch, up)
}
