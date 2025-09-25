package text

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/DaanHessen/walker-tui/internal/engine"
)

// Narrator produces scene and outcome prose.
type Narrator interface {
	Scene(ctx context.Context, st any) (string, error)
	Outcome(ctx context.Context, st any, ch any, up any) (string, error)
}

// DeepSeek wraps the DeepSeek Reasoner model for both narration and director planning.
type DeepSeek struct {
	apiKey string
	client *http.Client
	prompt string
}

func NewDeepSeek(apiKey string) (*DeepSeek, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("missing deepseek api key")
	}
	prompt, err := getSystemPrompt()
	if err != nil {
		return nil, err
	}
	return &DeepSeek{apiKey: apiKey, client: &http.Client{Timeout: 2 * time.Second}, prompt: prompt}, nil
}

func (d *DeepSeek) Scene(ctx context.Context, st any) (string, error) {
	state := sanitizeState(st)
	if tz, ok := state["timezone"].(string); ok {
		if ldt, ok2 := state["local_datetime"].(string); ok2 {
			state["context_hint"] = fmt.Sprintf("Local time %s (%s)", ldt, tz)
		}
	}
	prompt, err := buildScenePrompt(state)
	if err != nil {
		return "", err
	}
	messages := []dsMessage{
		{Role: "system", Content: d.prompt},
		{Role: "user", Content: prompt},
	}
	text, err := d.chat(ctx, messages, 700)
	if err != nil {
		return "", err
	}
	cleaned := sanitizeOutput(text)
	if !validateNarrative(cleaned, true, state) {
		return "", errors.New("scene validation failed")
	}
	return cleaned, nil
}

func (d *DeepSeek) Outcome(ctx context.Context, st any, ch any, up any) (string, error) {
	state := sanitizeState(st)
	if tz, ok := state["timezone"].(string); ok {
		if ldt, ok2 := state["local_datetime"].(string); ok2 {
			state["context_hint"] = fmt.Sprintf("Local time %s (%s)", ldt, tz)
		}
	}
	prompt, err := buildOutcomePrompt(state, ch, up)
	if err != nil {
		return "", err
	}
	messages := []dsMessage{
		{Role: "system", Content: d.prompt},
		{Role: "user", Content: prompt},
	}
	text, err := d.chat(ctx, messages, 520)
	if err != nil {
		return "", err
	}
	cleaned := sanitizeOutput(text)
	if !validateNarrative(cleaned, false, state) {
		return "", errors.New("outcome validation failed")
	}
	return cleaned, nil
}

func (d *DeepSeek) PlanEvent(ctx context.Context, req engine.DirectorRequest) (engine.DirectorPlan, error) {
	if len(req.Available) == 0 {
		return engine.DirectorPlan{}, errors.New("no available events")
	}
	prompt, err := buildDirectorPrompt(req)
	if err != nil {
		return engine.DirectorPlan{}, err
	}
	messages := []dsMessage{
		{Role: "system", Content: d.prompt},
		{Role: "user", Content: prompt},
	}
	raw, err := d.chat(ctx, messages, 600)
	if err != nil {
		return engine.DirectorPlan{}, err
	}
	cleaned := sanitizeOutput(raw)
	var resp directorResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return engine.DirectorPlan{}, fmt.Errorf("director json parse: %w", err)
	}
	if len(resp.Choices) < 2 || len(resp.Choices) > 6 {
		return engine.DirectorPlan{}, fmt.Errorf("director returned %d choices", len(resp.Choices))
	}
	plan := engine.DirectorPlan{
		EventID:   strings.TrimSpace(resp.EventID),
		EventName: strings.TrimSpace(resp.EventName),
		Guidance:  strings.TrimSpace(resp.Guidance),
	}
	for _, ch := range resp.Choices {
		plan.Choices = append(plan.Choices, engine.PlannedChoice{
			Label:     ch.Label,
			Archetype: ch.Archetype,
			Cost: engine.PlanCost{
				Time:    ch.BaseCost.Time,
				Fatigue: ch.BaseCost.Fatigue,
				Hunger:  ch.BaseCost.Hunger,
				Thirst:  ch.BaseCost.Thirst,
			},
			Risk: ch.BaseRisk,
		})
	}
	return plan, nil
}

type directorResponse struct {
	EventID   string                   `json:"event_id"`
	EventName string                   `json:"event_name"`
	Guidance  string                   `json:"guidance"`
	Choices   []directorChoiceResponse `json:"choices"`
}

type directorChoiceResponse struct {
	Label     string   `json:"label"`
	Archetype string   `json:"archetype"`
	BaseRisk  string   `json:"base_risk"`
	BaseCost  costSpec `json:"base_cost"`
}

type costSpec struct {
	Time    int `json:"time"`
	Fatigue int `json:"fatigue"`
	Hunger  int `json:"hunger"`
	Thirst  int `json:"thirst"`
}

func buildScenePrompt(state map[string]any) (string, error) {
	payload := map[string]any{
		"role":         "narrator",
		"instructions": "Write a single 120-250 word paragraph in second person, present tense. No lists, no odds, no mechanics or meta commentary. Stay grounded in the survivor's current perceptions only.",
		"state":        state,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func buildOutcomePrompt(state map[string]any, choice any, update any) (string, error) {
	payload := map[string]any{
		"role":         "outcome_narrator",
		"instructions": "Write a single 100-200 word paragraph in second person, present tense describing only immediate consequences. Weave in UPDATE deltas implicitly. No lists, no stats, no odds, no meta.",
		"state":        state,
		"choice":       choice,
		"update":       update,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func buildDirectorPrompt(req engine.DirectorRequest) (string, error) {
	stateJSON, err := json.MarshalIndent(req.State, "", "  ")
	if err != nil {
		return "", err
	}
	eventsJSON, err := json.MarshalIndent(req.Available, "", "  ")
	if err != nil {
		return "", err
	}
	historyJSON, err := json.MarshalIndent(req.History, "", "  ")
	if err != nil {
		return "", err
	}
	archetypes := engine.AllowedArchetypes()
	payload := map[string]any{
		"role":               "director",
		"instructions":       "Select one event from available_events and propose 2-6 choices. Respond with JSON only, matching schema {event_id, event_name, guidance, choices:[{label, archetype, base_cost:{time,fatigue,hunger,thirst}, base_risk}]}. Use only archetypes from allowed_archetypes.",
		"allowed_archetypes": archetypes,
		"available_events":   json.RawMessage(eventsJSON),
		"state":              json.RawMessage(stateJSON),
		"history":            json.RawMessage(historyJSON),
		"scene_index":        req.SceneIndex,
		"scarcity":           req.Scarcity,
		"text_density":       req.TextDensity,
		"difficulty":         req.Difficulty,
		"infected_local":     req.InfectedLocal,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

var (
	systemPromptOnce sync.Once
	cachedPrompt     string
	cachedPromptErr  error
)

func getSystemPrompt() (string, error) {
	systemPromptOnce.Do(func() {
		if override := strings.TrimSpace(os.Getenv("ZEROPOINT_PROMPT_PATH")); override != "" {
			data, err := os.ReadFile(override)
			if err != nil {
				cachedPromptErr = fmt.Errorf("load prompt override: %w", err)
				return
			}
			cachedPrompt = string(data)
			cachedPromptErr = nil
			return
		}
		wd, err := os.Getwd()
		if err != nil {
			cachedPromptErr = err
			return
		}
		candidates := []string{
			filepath.Join(wd, "docs", "instructions.md"),
			filepath.Join(wd, "..", "docs", "instructions.md"),
			filepath.Join(wd, "..", "..", "docs", "instructions.md"),
		}
		for _, path := range candidates {
			if data, err := os.ReadFile(path); err == nil {
				cachedPrompt = string(data)
				cachedPromptErr = nil
				return
			}
		}
		cachedPromptErr = fmt.Errorf("unable to load docs/instructions.md (tried %s)", strings.Join(candidates, ", "))
	})
	return cachedPrompt, cachedPromptErr
}

func (d *DeepSeek) chat(ctx context.Context, messages []dsMessage, maxTokens int) (string, error) {
	reqBody, err := json.Marshal(dsRequest{Model: "deepseek-reasoner", Messages: messages, MaxTokens: maxTokens})
	if err != nil {
		return "", err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		res, err := d.doRequest(ctx, reqBody)
		if err != nil {
			lastErr = err
			backoff(attempt)
			continue
		}
		return res, nil
	}
	if lastErr == nil {
		lastErr = errors.New("deepseek request failed")
	}
	return "", lastErr
}

// HTTP + response structs

type dsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dsRequest struct {
	Model     string      `json:"model"`
	Messages  []dsMessage `json:"messages"`
	MaxTokens int         `json:"max_tokens,omitempty"`
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

func (d *DeepSeek) doRequest(ctx context.Context, body []byte) (string, error) {
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

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var ctrlRegexp = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F]`)

func validateNarrative(s string, isScene bool, st map[string]any) bool {
	wc := wordCount(s)
	if isScene {
		if wc < 120 || wc > 250 {
			return false
		}
	} else {
		if wc < 100 || wc > 200 {
			return false
		}
	}
	ls := strings.ToLower(s)
	if strings.Contains(ls, "\n-") || strings.Contains(ls, "\n*") {
		return false
	}
	if strings.Contains(ls, "stat ") || strings.Contains(ls, "stats") {
		return false
	}
	infectedPresent := false
	if v, ok := st["infected_present"]; ok {
		infectedPresent = v == true || strings.EqualFold(fmt.Sprintf("%v", v), "true")
	}
	if !infectedPresent {
		for _, banned := range []string{"infected", "zombie", "horde"} {
			if strings.Contains(ls, banned) {
				return false
			}
		}
	}
	return true
}

func wordCount(s string) int { return len(strings.Fields(s)) }

func sanitizeOutput(s string) string {
	s = ansiRegexp.ReplaceAllString(s, "")
	s = ctrlRegexp.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
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
	time.Sleep(time.Duration(200+attempt*250) * time.Millisecond)
}

func hashPayload(payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	return sum[:], nil
}

func SceneCacheKey(state any) ([]byte, error) {
	return hashPayload(map[string]any{
		"kind":  "scene",
		"state": state,
	})
}

func OutcomeCacheKey(state any, choice engine.Choice, delta engine.Stats) ([]byte, error) {
	payload := map[string]any{
		"kind":  "outcome",
		"state": state,
		"choice": map[string]any{
			"id":        choice.ID,
			"label":     choice.Label,
			"archetype": choice.Archetype,
			"cost":      choice.Cost,
			"risk":      choice.Risk,
			"custom":    choice.Custom,
		},
		"delta": delta,
	}
	return hashPayload(payload)
}
