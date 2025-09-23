package text

import (
	"context"
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
	return "[Template scene placeholder]", nil
}
func (t *templateNarrator) Outcome(ctx context.Context, st any, ch any, up any) (string, error) {
	return "[Template outcome placeholder]", nil
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
