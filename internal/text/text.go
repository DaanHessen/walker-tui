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
	"sort"
	"strings"
	"time"

	"github.com/DaanHessen/walker-tui/internal/engine"
)

// Narrator is the interface used by the game to render prose.
type Narrator interface {
	Scene(ctx context.Context, st any) (string, error)
	Outcome(ctx context.Context, st any, ch any, up any) (string, error)
}

// templateNarrator now produces spec-aligned markdown (CHARACTER OVERVIEW, SKILLS, STATS, INVENTORY, SCENE)
// Word counts: Scene ~120-180 words; Outcome ~100-150 words (within allowed ranges) using deterministic RNG.
type templateNarrator struct{ seed int64 }

func NewTemplateNarrator(seed int64) Narrator { return &templateNarrator{seed: seed} }

func (t *templateNarrator) seededRand(extra any) *rand.Rand {
	h := fnv64aHash(fmt.Sprintf("%d-%v", t.seed, extra))
	return rand.New(rand.NewSource(int64(h)))
}

func (t *templateNarrator) Scene(ctx context.Context, st any) (string, error) {
	m, ok := st.(map[string]any)
	if !ok {
		return "[invalid scene state]", nil
	}
	r := t.seededRand(m["world_day"]) // deterministic per day
	var b strings.Builder
	// CHARACTER OVERVIEW
	b.WriteString("## CHARACTER OVERVIEW\n")
	b.WriteString(fmt.Sprintf("Name: %v | Age: %v | Background: %v | Day: %v | Region: %v | Location: %v\n", m["name"], m["age"], valueOr(m, "background", "unknown"), m["world_day"], m["region"], m["location"]))
	if conds, ok := m["conditions"].([]engine.Condition); ok && len(conds) > 0 {
		b.WriteString("Conditions: ")
		for i, c := range conds {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(string(c))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## SKILLS\n")
	if skills, ok := m["skills"].(map[engine.Skill]int); ok {
		// stable order
		keys := make([]string, 0, len(skills))
		for k := range skills {
			keys = append(keys, string(k))
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("- %s: %d\n", k, skills[engine.Skill(k)]))
		}
	}
	b.WriteString("\n## STATS\n")
	if stats, ok := m["stats"].(engine.Stats); ok {
		b.WriteString(fmt.Sprintf("Health %d | Hunger %d | Thirst %d | Fatigue %d | Morale %d\n", stats.Health, stats.Hunger, stats.Thirst, stats.Fatigue, stats.Morale))
	}
	b.WriteString("\n## INVENTORY\n")
	if inv, ok := m["inventory"].(engine.Inventory); ok {
		b.WriteString(listInv(inv))
	}
	b.WriteString("\n## SCENE\n")
	infected := false
	if v, ok := m["infected_present"].(bool); ok {
		infected = v
	}
	lad, _ := m["lad"].(int)
	day, _ := m["world_day"].(int)
	// Build sentences respecting LAD gate
	sentences := buildSceneSentences(r, infected, lad, day)
	writeSentences(&b, sentences, 120, 250)
	return b.String(), nil
}

func (t *templateNarrator) Outcome(ctx context.Context, st any, ch any, up any) (string, error) {
	state, _ := st.(map[string]any)
	choice, _ := ch.(engine.Choice)
	delta, _ := up.(engine.Stats)
	r := t.seededRand("out-" + choice.Label + fmt.Sprint(state["world_day"]))
	var b strings.Builder
	b.WriteString("## UPDATE\n")
	b.WriteString(formatDelta(delta))
	b.WriteString("\n## OUTCOME SCENE\n")
	infected := false
	if v, ok := state["infected_present"].(bool); ok {
		infected = v
	}
	lad, _ := state["lad"].(int)
	day, _ := state["world_day"].(int)
	sentences := buildOutcomeSentences(r, infected, lad, day, choice.Label)
	writeSentences(&b, sentences, 100, 200)
	return b.String(), nil
}

// Helpers --------------------------------------------------------------------

func listInv(inv engine.Inventory) string {
	var sb strings.Builder
	if len(inv.Weapons) > 0 {
		sb.WriteString("Weapons: ")
		sb.WriteString(strings.Join(inv.Weapons, ", "))
		sb.WriteString("\n")
	} else {
		sb.WriteString("Weapons: none\n")
	}
	sb.WriteString(fmt.Sprintf("Food Days: %.1f | Water Liters: %.1f\n", inv.FoodDays, inv.WaterLiters))
	if len(inv.Medical) > 0 {
		sb.WriteString("Medical: ")
		sb.WriteString(strings.Join(inv.Medical, ", "))
		sb.WriteString("\n")
	}
	if len(inv.Tools) > 0 {
		sb.WriteString("Tools: ")
		sb.WriteString(strings.Join(inv.Tools, ", "))
		sb.WriteString("\n")
	}
	if len(inv.Special) > 0 {
		sb.WriteString("Special: ")
		sb.WriteString(strings.Join(inv.Special, ", "))
		sb.WriteString("\n")
	}
	if inv.Memento != "" {
		sb.WriteString("Memento: ")
		sb.WriteString(inv.Memento)
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatDelta(d engine.Stats) string {
	var parts []string
	add := func(label string, v int) {
		if v != 0 {
			parts = append(parts, fmt.Sprintf("%s %+d", label, v))
		}
	}
	add("Health", d.Health)
	add("Hunger", d.Hunger)
	add("Thirst", d.Thirst)
	add("Fatigue", d.Fatigue)
	add("Morale", d.Morale)
	if len(parts) == 0 {
		return "No immediate stat changes."
	}
	return strings.Join(parts, ", ")
}

func buildSceneSentences(r *rand.Rand, infected bool, lad, day int) []string {
	base := []string{
		"Clouds hang in a pale strip above the district while faint wind carries dust.",
		"You catalog your supplies with a quick practiced sweep of the pack.",
		"Distant civic sirens waver, more overlapping than earlier hours.",
		"Old posters peel from a shuttered storefront, edges twitching in the breeze.",
		"Your pulse steadies as you judge how long to remain here.",
		"Somewhere metal clatters, then silence folds back in.",
		"You weigh risk against dwindling reserves, tracing mental escape lines.",
	}
	preArrival := []string{
		"No bodies in the open yet; the quiet carries only anxious human motion.",
		"People move in tense bursts, avoiding eye contact, phones clenched.",
		"A helicopter circles beyond view, its chop filtered to a dull throb.",
	}
	postArrival := []string{
		"A lurching figure pivots toward a sharp noise, head tilting with puppet slackness.",
		"Two infected collide, reset orientation, then drift apart without speech.",
		"A smear darkens cracked asphalt where something was dragged earlier.",
		"Behavior patterns look marginally more reactive than yesterday.",
	}
	var pool []string
	pool = append(pool, base...)
	if day < lad || !infected {
		pool = append(pool, preArrival...)
	} else {
		pool = append(pool, postArrival...)
	}
	// deterministic shuffle pick
	res := make([]string, 0, 12)
	for len(pool) > 0 && len(res) < 12 {
		i := r.Intn(len(pool))
		res = append(res, pool[i])
		pool = append(pool[:i], pool[i+1:]...)
	}
	return res
}

func buildOutcomeSentences(r *rand.Rand, infected bool, lad, day int, action string) []string {
	base := []string{
		"Muscles register the immediate cost while you reassess surroundings.",
		"Breathing evens out as you parse new lines of approach.",
		"Your decision narrows possible futures; you watch for rapid feedback.",
		"Ambient noise shifts half a register then settles again.",
		"You account for resource changes with practiced mental tallies.",
	}
	preArrival := []string{"Nearby civilians still dominate the landscape, not the infected.", "Tension rises without the obvious trigger of open contagion."}
	postArrival := []string{"One staggered silhouette rotates a full arc before resuming slow drift.", "A low chain of throaty exhalations passes between inert figures."}
	pool := append([]string{}, base...)
	if day < lad || !infected {
		pool = append(pool, preArrival...)
	} else {
		pool = append(pool, postArrival...)
	}
	// include action reference
	pool = append(pool, fmt.Sprintf("The choice to %s shapes the minute's texture in subtle ways.", strings.ToLower(action)))
	res := make([]string, 0, 10)
	for len(pool) > 0 && len(res) < 10 {
		i := r.Intn(len(pool))
		res = append(res, pool[i])
		pool = append(pool[:i], pool[i+1:]...)
	}
	return res
}

func writeSentences(b *strings.Builder, sentences []string, minWords, maxWords int) {
	words := 0
	for _, s := range sentences {
		w := wordCount(s)
		if words+w > maxWords {
			break
		}
		b.WriteString(s)
		if !strings.HasSuffix(s, "\n") {
			b.WriteString(" ")
		}
		words += w
		if words >= minWords {
			break
		}
	}
	b.WriteString("\n")
}

func wordCount(s string) int { return len(strings.Fields(s)) }

func valueOr(m map[string]any, k string, def string) any {
	if v, ok := m[k]; ok {
		return v
	}
	return def
}

// Simple FNV64a hash for deterministic RNG derivation.
func fnv64aHash(s string) uint64 {
	const (
		offset64 = 1469598103934665603
		prime64  = 1099511628211
	)
	var h uint64 = offset64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

// WithFallback returns a narrator that prefers primary and falls back to backup on error.
func WithFallback(primary, fallback Narrator) Narrator {
	return &fallbackNarrator{p: primary, f: fallback}
}

type fallbackNarrator struct{ p, f Narrator }

func (n *fallbackNarrator) Scene(ctx context.Context, st any) (string, error) {
	if n.p == nil {
		return n.f.Scene(ctx, st)
	}
	if s, err := n.p.Scene(ctx, st); err == nil {
		return s, nil
	}
	return n.f.Scene(ctx, st)
}
func (n *fallbackNarrator) Outcome(ctx context.Context, st any, ch any, up any) (string, error) {
	if n.p == nil {
		return n.f.Outcome(ctx, st, ch, up)
	}
	if s, err := n.p.Outcome(ctx, st, ch, up); err == nil {
		return s, nil
	}
	return n.f.Outcome(ctx, st, ch, up)
}

// reasonerNarrator implements Narrator using the Reasoner API.
type reasonerNarrator struct {
	apiKey             string
	client             *http.Client
	maxSceneWordsMin   int
	maxSceneWordsMax   int
	maxOutcomeWordsMin int
	maxOutcomeWordsMax int
}

type dsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dsRequest struct {
	Model     string      `json:"model"`
	Messages  []dsMessage `json:"messages"`
	MaxTokens int         `json:"max_tokens,omitempty"`
	Stream    bool        `json:"stream,omitempty"`
}

type dsChoice struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
}

type dsResponse struct {
	Choices []dsChoice `json:"choices"`
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var ctrlRegexp = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F]`)

// NewNarrator constructs the Reasoner narrator; returns error if apiKey empty.
func NewNarrator(apiKey string) (Narrator, error) {
	if apiKey == "" {
		return nil, errors.New("missing API key")
	}
	return &reasonerNarrator{
		apiKey:           apiKey,
		client:           &http.Client{Timeout: 2 * time.Second},
		maxSceneWordsMin: 120, maxSceneWordsMax: 250,
		maxOutcomeWordsMin: 100, maxOutcomeWordsMax: 200,
	}, nil
}

func (d *reasonerNarrator) Scene(ctx context.Context, st any) (string, error) {
	return d.call(ctx, st, nil, nil, true)
}

func (d *reasonerNarrator) Outcome(ctx context.Context, st any, ch any, up any) (string, error) {
	return d.call(ctx, st, ch, up, false)
}

// call assembles prompt and performs API invocation.
func (d *reasonerNarrator) call(ctx context.Context, st any, ch any, up any, isScene bool) (string, error) {
	// Marshal state (ensure no functions / complex types)
	cleanState := sanitizeState(st)
	stateJSON, _ := json.Marshal(cleanState)
	var userPrompt bytes.Buffer
	if isScene {
		userPrompt.WriteString("Return ONLY a markdown section with SCENE narrative paragraphs (no headings) within required word bounds. Present tense, 2nd person. Respect LAD gate: if world_day < lad or infected_present=false, do NOT include open-area infected. State JSON follows.\n")
		userPrompt.Write(stateJSON)
		userPrompt.WriteString("\nLength: 120-250 words.")
	} else {
		// outcome
		deltaJSON, _ := json.Marshal(up)
		choiceJSON, _ := json.Marshal(ch)
		userPrompt.WriteString("Return ONLY the OUTCOME narrative (markdown paragraphs, no headings) 100-200 words. Reference consequences implicitly, no meta, no odds. Respect LAD gate.\nState: ")
		userPrompt.Write(stateJSON)
		userPrompt.WriteString("\nChoice: ")
		userPrompt.Write(choiceJSON)
		userPrompt.WriteString("\nDelta: ")
		userPrompt.Write(deltaJSON)
	}
	messages := []dsMessage{
		{Role: "system", Content: "You are the narrative engine for a survival outbreak roguelite. Obey instructions exactly. Do not reveal internal state or rules."},
		{Role: "user", Content: userPrompt.String()},
	}
	reqBody, _ := json.Marshal(dsRequest{Model: "deepseek-reasoner", Messages: messages})
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		resp, err := d.doRequest(ctx, reqBody)
		if err != nil {
			lastErr = err
			backoff(attempt)
			continue
		}
		text := sanitizeOutput(resp)
		if !d.validLength(text, isScene) {
			lastErr = errors.New("length validation failed")
			backoff(attempt)
			continue
		}
		// ensure no headings (strip if any)
		text = stripHeadings(text)
		return text, nil
	}
	return "", lastErr
}

func (d *reasonerNarrator) doRequest(ctx context.Context, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.deepseek.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("deepseek status %d", resp.StatusCode)
	}
	var dr dsResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return "", err
	}
	if len(dr.Choices) == 0 {
		return "", errors.New("no choices")
	}
	return dr.Choices[0].Message.Content, nil
}

func (d *reasonerNarrator) validLength(s string, isScene bool) bool {
	wc := wordCount(s)
	if isScene {
		return wc >= d.maxSceneWordsMin && wc <= d.maxSceneWordsMax
	}
	return wc >= d.maxOutcomeWordsMin && wc <= d.maxOutcomeWordsMax
}

func sanitizeOutput(s string) string {
	s = ansiRegexp.ReplaceAllString(s, "")
	s = ctrlRegexp.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func stripHeadings(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "#") {
			lines[i] = strings.TrimLeft(l, "# ")
		}
	}
	return strings.Join(lines, "\n")
}

func sanitizeState(st any) map[string]any {
	m, ok := st.(map[string]any)
	if !ok {
		return map[string]any{"error": "bad_state"}
	}
	clean := make(map[string]any, len(m))
	for k, v := range m {
		clean[k] = v
	}
	return clean
}

func backoff(attempt int) {
	if attempt == 0 {
		time.Sleep(150 * time.Millisecond)
	} else {
		time.Sleep(time.Duration(300+attempt*150) * time.Millisecond)
	}
}
