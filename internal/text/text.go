package text

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/DaanHessen/walker-tui/internal/engine"
)

// Narrator interface unchanged.
type Narrator interface {
	Scene(ctx context.Context, st any) (string, error)
	Outcome(ctx context.Context, st any, ch any, up any) (string, error)
}

// MinimalFallbackNarrator: emergency only, neutral concise sentences.
type MinimalFallbackNarrator struct{}

func NewMinimalFallbackNarrator() Narrator { return &MinimalFallbackNarrator{} }

func (m *MinimalFallbackNarrator) Scene(ctx context.Context, st any) (string, error) {
	state, _ := st.(map[string]any)
	var parts []string
	grab := func(k string) string { if v, ok := state[k]; ok { return fmt.Sprintf("%v", v) }; return "" }
	region := grab("region")
	tod := grab("time_of_day")
	weather := grab("weather")
	season := grab("season")
	lad := grab("lad")
	worldDay := grab("world_day")
	infected := grab("infected_present")
	inv := "limited supplies"
	if invMap, ok := state["inventory"].(engine.Inventory); ok {
		inv = fmt.Sprintf("%.1fd food, %.1fL water", invMap.FoodDays, invMap.WaterLiters)
	}
	parts = append(parts, fmt.Sprintf("You are in %s this %s. The %s weather in %s feels typical for %s.", region, tod, weather, season, season))
	parts = append(parts, fmt.Sprintf("World day %s; local arrival day threshold %s.", worldDay, lad))
	if infected == "true" || infected == "1" || strings.EqualFold(fmt.Sprintf("%v", infected), "true") {
		parts = append(parts, "Infected activity is now a possibility; you stay aware of movement and sound.")
	} else {
		parts = append(parts, "Open areas remain unnervingly quiet; no infected are visible yet.")
	}
	parts = append(parts, fmt.Sprintf("Your current provisions are %s.", inv))
	parts = append(parts, "You weigh immediate needs against risk.")
	return strings.Join(parts, " "), nil
}

func (m *MinimalFallbackNarrator) Outcome(ctx context.Context, st any, ch any, up any) (string, error) {
	choice, _ := ch.(engine.Choice)
	delta, _ := up.(engine.Stats)
	var segs []string
	segs = append(segs, fmt.Sprintf("You commit to '%s'.", choice.Label))
	// reference deltas implicitly
	if delta.Fatigue > 0 {
		segs = append(segs, "The effort leaves you a little more tired.")
	} else if delta.Fatigue < 0 {
		segs = append(segs, "You feel a touch more rested.")
	}
	if delta.Hunger < 0 || delta.Thirst < 0 {
		segs = append(segs, "Some basic needs ease slightly.")
	}
	if delta.Health < 0 {
		segs = append(segs, "You take a minor hit to your well-being.")
	}
	if delta.Morale != 0 {
		if delta.Morale > 0 { segs = append(segs, "Your resolve steadies a little.") } else { segs = append(segs, "Your mood dips.") }
	}
	segs = append(segs, "You reassess your immediate options.")
	return strings.Join(segs, " "), nil
}

// Fallback wrapper.
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

// DeepSeek Reasoner narrator implementation.
type deepSeekNarrator struct {
	apiKey string
	client *http.Client
}

func NewDeepSeekNarrator(apiKey string) (Narrator, error) {
	if apiKey == "" { return nil, errors.New("missing api key") }
	return &deepSeekNarrator{apiKey: apiKey, client: &http.Client{Timeout: 2 * time.Second}}, nil
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var ctrlRegexp = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F]`)

func (d *deepSeekNarrator) Scene(ctx context.Context, st any) (string, error) { return d.call(ctx, st, nil, nil, true) }
func (d *deepSeekNarrator) Outcome(ctx context.Context, st any, ch any, up any) (string, error) { return d.call(ctx, st, ch, up, false) }

// call constructs prompts per spec.
func (d *deepSeekNarrator) call(ctx context.Context, st any, ch any, up any, isScene bool) (string, error) {
	cleanState := sanitizeState(st)
	stateJSON, _ := json.Marshal(cleanState)
	var userJSON bytes.Buffer
	userJSON.Write(stateJSON)
	var messages []dsMessage
	if isScene {
		messages = []dsMessage{
			{Role: "system", Content: "You are the narrator for a grounded survival TUI game. Write a single **120–250 word** paragraph in **second person, present tense**, strictly from the survivor's current perspective. **No meta, no rules, no statistics, no odds, no headings or lists.** Do not invent items, skills, or conditions. Respect that **open-area infected are not present if the given LAD has not been reached**. Maintain realism and sensory detail."},
			{Role: "user", Content: userJSON.String()},
		}
	} else {
		choiceJSON, _ := json.Marshal(ch)
		deltaJSON, _ := json.Marshal(up)
		combined := fmt.Sprintf("{\n\"state\": %s,\n\"choice\": %s,\n\"update\": %s\n}", userJSON.String(), string(choiceJSON), string(deltaJSON))
		messages = []dsMessage{
			{Role: "system", Content: "Write a single **100–200 word** paragraph in second person, present tense, strictly from the survivor's perspective. Describe only the immediate consequences of the chosen action. **No meta, no rules, no statistics, no odds, no headings or lists.** Do not invent items or mechanics. Subtly reflect the provided UPDATE deltas. Obey the LAD gate."},
			{Role: "user", Content: combined},
		}
	}
	reqBody, _ := json.Marshal(dsRequest{Model: "deepseek-reasoner", Messages: messages})
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		select { case <-ctx.Done(): return "", ctx.Err(); default: }
		resp, err := d.doRequest(ctx, reqBody)
		if err != nil { lastErr = err; backoff(attempt); continue }
		text := sanitizeOutput(resp)
		if !validateNarrative(text, isScene) { lastErr = errors.New("validation failed"); backoff(attempt); continue }
		return text, nil
	}
	return "", lastErr
}

// HTTP + response structs

type dsMessage struct { Role string `json:"role"`; Content string `json:"content"` }
type dsRequest struct { Model string `json:"model"`; Messages []dsMessage `json:"messages"`; MaxTokens int `json:"max_tokens,omitempty"` }
type dsChoice struct { Message struct { Role string `json:"role"`; Content string `json:"content"` } `json:"message"` }
type dsResponse struct { Choices []dsChoice `json:"choices"` }

func (d *deepSeekNarrator) doRequest(ctx context.Context, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.deepseek.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil { return "", err }
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode >= 300 { return "", fmt.Errorf("deepseek status %d", resp.StatusCode) }
	var dr dsResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil { return "", err }
	if len(dr.Choices) == 0 { return "", errors.New("no choices") }
	return dr.Choices[0].Message.Content, nil
}

// Validation ---------------------------------------------------------
func validateNarrative(s string, isScene bool) bool {
	wc := wordCount(s)
	if isScene {
		if wc < 120 || wc > 250 { return false }
	} else {
		if wc < 100 || wc > 200 { return false }
	}
	if strings.Contains(s, "\n-") || strings.Contains(s, "\n*") { return false }
	if strings.Contains(strings.ToLower(s), "stat") { return false }
	return true
}

func wordCount(s string) int { return len(strings.Fields(s)) }

// sanitize / helpers -------------------------------------------------
func sanitizeOutput(s string) string {
	s = ansiRegexp.ReplaceAllString(s, "")
	s = ctrlRegexp.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func sanitizeState(st any) map[string]any {
	m, ok := st.(map[string]any); if !ok { return map[string]any{"error":"bad_state"} }
	clean := make(map[string]any, len(m))
	for k,v := range m { clean[k]=v }
	return clean
}

func backoff(attempt int) { time.Sleep(time.Duration(200+attempt*250) * time.Millisecond) }

// Utility deterministic sampling (used by fallback maybe later)
func sample[T any](r *rand.Rand, in []T, n int) []T { if n>len(in){n=len(in)}; out:=make([]T,0,n); perm:=r.Perm(len(in)); for i:=0;i<n;i++{out=append(out,in[perm[i]])}; return out }
