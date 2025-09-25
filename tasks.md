# tasks.md — Zero Point / walker-tui • Complete Build Plan

> **Scope**: Finish the TUI roguelike “Zero Point / walker-tui”. Postgres-only. DeepSeek-Reasoner for narration (with a tiny emergency fallback). Deterministic **seeding** and a **data-driven event system** drive gameplay. Enforce LAD gating, difficulty drains, five core conditions, timezone-correct UI, and strict layout.  
> **Rule**: Do not restructure folders/architecture unless a bug fix requires it. Prefer surgical, testable changes.

---

## 0) Ground Rules (apply to all work)
- **Determinism**: No gameplay logic uses wall clock or global `math/rand`. All randomness comes from the seeded PRNG substreams.
- **Transactions**: One DB transaction per turn (insert scene → choices → update → outcome → survivor state).
- **Safety**: Clamp meters to 0..100, skills to 0..5. Enforce enum validation. Context timeouts for DB/HTTP. No panics in normal flow.
- **Narration**: DeepSeek only when key present; fallback is a **tiny**, deterministic, neutral paragraph builder (no “template narrator” that invents mechanics).
- **Fail fast**: If the event registry or migrations are missing, **abort** with a clear error; do not silently fall back to another gameplay loop.
- **UI contract**: Strict layout & ordering. No meta reveals (origin site, odds, internal state).

---

## M0 — Sanity & Cleanup
**Goal**: Remove any code paths that contradict determinism/specs; set the stage for new features.

**Tasks**
- Search & remove any “event-system-unavailable” or “template narrator” fallbacks that alter gameplay. Replace with explicit init/runtime errors.
- Grep for `time.Now()`, `rand.Seed`, `math/rand` global usage; quarantine for later replacement in M2.
- Ensure error paths do not swallow errors; wrap with context.

**Acceptance**
- The game aborts early with helpful error if migrations or event registry are missing.
- No gameplay branch depends on wall clock or global PRNG.

---

## M1 — Schema Migrations (Postgres-only)
**Goal**: Persist seed (TEXT), rules version, difficulty, event tracking, and narration cache.

**Files**
- `db/migrations/0004_settings_difficulty.(up|down).sql`
- `db/migrations/0005_run_seed_rules.(up|down).sql`
- `db/migrations/0006_events_schema.(up|down).sql`

**Tables/Changes**
1. `settings`  
   - `difficulty TEXT NOT NULL DEFAULT 'standard' CHECK (difficulty IN ('easy','standard','hard'))`
2. `runs`  
   - Add `seed TEXT` (canonical).  
   - Add `rules_version TEXT NOT NULL DEFAULT '1.0.0'`.  
   - If `runs.seed` is BIGINT exists → rename to `seed_int` and **backfill** `seed = seed_int::text` (idempotent).
3. `event_instances`  
   - Columns: `run_id UUID, survivor_id UUID, event_id TEXT, world_day INT, scene_idx INT, cooldown_until_scene INT, arc_id TEXT NULL, arc_step INT NULL, once_fired BOOL DEFAULT FALSE, PRIMARY KEY (run_id, event_id, scene_idx)`.
4. `narration_cache` (optional but recommended)  
   - Columns: `run_id UUID, state_hash BYTEA, kind TEXT CHECK (kind IN ('scene','outcome')), text TEXT, PRIMARY KEY (run_id, state_hash, kind))`.

**Acceptance**
- Migrations run idempotently (twice) without errors.
- New columns visible and writable.

---

## M2 — Deterministic PRNG & Seeding UX
**Goal**: Minecraft-style seeds; reproducible mechanics; UX for seed entry.

**Files**
- `internal/engine/prng.go` (new)
- Update stores for `runs.seed` & `runs.rules_version`
- UI: New Game flow / About & Settings

**Tasks**
- Implement:
  - `SeedFromString(string) uint64` (FNV-1a or xxhash).
  - SplitMix64 or xoshiro PRNG.
  - `Derive(base uint64, label string) uint64` (HMAC-SHA256 over `[seed || runID || rules_version || label]`).
- Define substreams:  
  `origin@rules:<ver>`, `survivor#<n>`, `day:<d>:turn:<t>:event|risk|loot|delta|narration`.
- New Game prompts for **Seed (optional)**; generate slug if empty; persist to `runs.seed`; display “Seed & Rules” in About/Settings; lock reseeding after run start.

**Acceptance**
- Same seed + same choices + same rules version → same origin, spawns, event selections, and deltas.
- Seed is visible in About/Rules. No global PRNG used for gameplay.

---

## M3 — Event Registry & Deterministic Selector
**Goal**: Engine chooses **what happens**; AI narrates. All events are data-driven.

**Files**
- `assets/events/*.yaml` (new pack)
- `internal/engine/events.go` (types, loader, selector)
- Integrate into `GenerateChoices` pipeline

**Event YAML Schema**
```yaml
id: "checkpoint_fracas_v1"
name: "Checkpoint Fracas"
tags: [urban, crowd, authority]
tier: pre_arrival            # pre_arrival | post_arrival | researcher
rarity: uncommon             # common=5, uncommon=3, rare=1 (weight)
cooldown_scenes: 12
once_per_run: false
preconditions:
  lad_relation: "<"          # "<" means only if world_day < LAD
  location_types: [city, suburb]
  time_of_day: [day, dusk]
  min_day: 0
  max_day: 7
  forbid_conditions: [bleeding, hypothermia]
effects_on_select:
  morale: -2                 # optional stage-setting deltas
choices:
  - id: "wait_and_watch"
    label: "Blend into the crowd and listen."
    archetypes: [scout, observe]
    base_risk: low
    base_cost: { time_min: 30, fatigue: +1 }
  - id: "negotiate_with_guard"
    label: "Approach the guards; ask about routes."
    archetypes: [negotiate]
    base_risk: moderate
    base_cost: { time_min: 20, fatigue: +1, visibility: +1 }
    gating:
      skills_any_min: [{ skill: leadership, min: 3 }]
  - id: "side_alley_probe"
    label: "Slip into a side alley to inspect barriers."
    archetypes: [stealth, scout]
    base_risk: moderate
    base_cost: { time_min: 25, fatigue: +2, noise: +1 }
outcome_deltas:
  wait_and_watch: { morale: "+1..+2" }
  negotiate_with_guard: { morale: "-1..+2" }
  side_alley_probe: { fatigue: "+1..+3" }
arc:
  id: "city_lockdown_arc"
  step: 1
  next_min_delay_scenes: 2
  next_candidates: ["blocked_route_v1","alley_encounter_v2"]
````

**Tasks**

* Loader: read `/assets/events/*.yaml`, validate schema.
* Selector: filter by preconditions (incl. LAD relation), apply rarity weights, drop cooldown/once-per-run, then **pick via PRNG substream**.
* Integrate: `GenerateChoices` uses selected event’s choices (2–6). Store an `event_instances` row for cooldown/once-per-run tracking.

**Acceptance**

* With the same seed and choices, event IDs selected per turn are reproducible.
* Pre-arrival scenes never select infected-in-open events.

---

## M4 — Mechanics Alignment

**Goal**: Enforce key rules: skills, first-survivor, LAD gating, difficulty drains.

**Tasks**

* **Skills**: clamp to **0–5 ints**; `GainSkill` → +1 only on meaningful practice; risk math reads 0–5.
* **First survivor**: spawn ≤100 km from origin on **Day 0**; **5% researcher** at **Day −9..0** inside facility; persist origin (never display).
* **Difficulty**: baseline per-scene drains (easy: `+1/+2/+1`; std: `+2/+3/+2`; hard: `+3/+4/+3`) in addition to choice costs.
* **LAD gating**: implement modifiers (hub −2, rural +2..+5, closures +2..+7, evac −1..−2) and enforce “no open-area infected pre-arrival” in both selection and narration preflight.

**Acceptance**

* First survivor rule and 5% researcher path observed.
* Baseline drains applied each turn per difficulty.
* No infected in open areas pre-arrival.

---

## M5 — Conditions (five core)

**Goal**: Implement triggers, drains, removals, and risk effects.

**Conditions**

1. **Bleeding**: H −4/−6/−8; Fatigue +2; +1 risk physical; remove by bandage/first-aid.
2. **Dehydration**: trigger Thirst ≥80 for 3 scenes; Fatigue +2; Morale −2; Health 0/−1/−1; remove by ≥0.5 L twice within 2 scenes & Thirst ≤40.
3. **Fever**: Fatigue +1; Morale −2; Health 0/−1/−1; +1 risk precision/stealth/medicine; remove by antipyretic/antibiotic + rest 6 scenes (within last 8).
4. **Hypothermia**: trigger cold/freezing + (wet or outdoors) for 3 scenes; H −2/−3/−4; Fatigue +2; +1 risk melee/precision/stealth; remove by warm/dry 4 consecutive scenes.
5. **Exhaustion**: trigger Fatigue ≥85; disable high-exertion options; +1 risk physical; if ≥4 scenes continuous: H −1/−1/−2; remove when Fatigue ≤50.

**Acceptance**

* Conditions can be acquired and cleared per rules; effects applied each scene; high-exertion disabled during exhaustion.

---

## M6 — DeepSeek Narrator Only + Minimal Fallback + Cache

**Goal**: Enforce prompt contracts and keep a tiny emergency path.

**Files**

* `internal/text/text.go` (or equivalent narrator)
* (optional) `narration_cache` repo

**Tasks**

* Remove any rich “template” narrator; keep **MinimalFallbackNarrator**: 4–6 neutral sentences (Scene), 3–5 (Outcome), deterministic phrasing using seed; no invented items; obey LAD.
* Prompts (code-enforced):

  * **Scene**: One **120–250 word** paragraph; 2nd person, present tense; no meta/lists/headings/odds; obey LAD; no invented mechanics.
  * **Outcome**: One **100–200 word** paragraph; immediate consequences; reflect UPDATE deltas; same constraints.
* Client: timeout ≤2s; 2 retries; sanitize ANSI; validate no headings/lists and length; on invalid → fallback.
* (Optional) Cache narration keyed by `(run_id, state_hash, kind)`.

**Acceptance**

* With DeepSeek key present, all narration uses DeepSeek. When failing, fallback kicks in with neutral text.

---

## M7 — TUI Layout Finalization

**Goal**: Implement strict UI structure.

**Tasks**

* **Top bar** (always visible):
  `ZERO POINT • <Region> • <Local YYYY-MM-DD HH:mm TZ> • <Season> • <TempBand>` … right: `World Day` and `[LAD:n]` **only when debug on** (env `ZEROPOINT_DEBUG_LAD=1`; toggle with F6).
* **Right sidebar** (fixed width): avatar placeholder; Identity (Name, Age, Background, Group+size); Traits (2–3); **full skills grid 0–5**; Stat bars (health,hunger,thirst,fatigue,morale) + Body Temp; Condition badges; Inventory sections (Weapons(+ammo), Food days, Water L, Medical, Tools, Special, Memento).
* **Main pane** (Glamour): Per turn **strict order**—Overview → Skills → Stats → Inventory → Scene → Choices (2–6 with `Cost: … | Risk: …`).
* **Bottom bar**: key hints `Tab panels • L Log • A Archive • S Settings • M Menu • ? Help • Q Quit`; **one-line Custom Action** input beneath; enable/disable with tooltip; 2-scene cooldown indicator.

**Acceptance**

* Visual layout matches spec; sections render in exact order; custom action behaves per rules.

---

## M8 — Timezone & Local Time

**Goal**: Correct local time rendering on survivor swap.

**Tasks**

* Store survivor’s IANA timezone in `environment` JSON.
* Compute local datetime from world day + TZ; display in top bar.
* After death → new survivor in new region/TZ; update top bar; keep **world day** steady.

**Acceptance**

* Top bar time/date reflects local TZ; swapping survivors updates TZ while world day persists.

---

## M9 — Custom Action System (final)

**Goal**: Robust free-text actions aligned to event choices.

**Tasks**

* Keep 2-scene cooldown and gating by fatigue/critical needs.
* Map text → nearest archetype for current **event context**; apply same cost/risk envelope; if infeasible, generate brief rejection outcome (no deltas).
* Disabled state: show tooltip (“Custom action unavailable now”).

**Acceptance**

* Valid custom actions resolve like analogous listed options; infeasible ones reject safely; cooldown enforced.

---

## M10 — Tests

**Goal**: Cover determinism and core rules.

**Tests**

* Seeding determinism: same seed + choices + rules version → same origin/spawns/events/deltas.
* Event selection: preconditions, rarity weights, cooldown, once-per-run.
* Choice invariant: always 2–6 options (with/without custom action).
* LAD gating: pre-arrival blocks open-area infected events and narration hints.
* Difficulty drains: per-scene baseline matches table.
* Conditions: each trigger/removal, drains, and risk mods.
* Skills clamp and `GainSkill` +1 only.
* First survivor ≤100 km; 5% researcher Day −9..0.
* Migrations idempotence.

**Acceptance**

* All tests pass locally; CI (if present) green.

---

## M11 — Docs & Final QA

**Goal**: Ship-quality polish.

**Tasks**

* Update README: install, migrations, settings, seeding, event packs, controls.
* Add `make` targets: `make db-up`, `make migrate-up`, `make enums`, `make build` → produces `walker-tui`.
* Verify acceptance checklist below.

---

## Acceptance Checklist (final)

* `walker-tui` opens Main Menu; New Game prompts optional seed; About shows “Seed & Rules”.
* First survivor near origin (≤100 km) on Day 0 (or 5% researcher Day −9..0); origin never printed.
* Event registry powers all choices; deterministic selection/cooldown; no “generic” fallback loops.
* Strict UI layout; local TZ time; `[LAD:n]` shown only with debug toggle.
* Pre-arrival scenes contain **no open-area infected**; LAD modifiers applied.
* Difficulty baseline drains in effect; five conditions implemented and influencing risks/drains; high-exertion disabled under Exhaustion.
* Custom actions map to archetypes, respect costs/risks, and enforce cooldown; safe rejection when infeasible.
* DeepSeek narrates; on failure, tiny fallback narrator produces neutral paragraphs; optional narration cache works.
* Tests cover determinism and rules; migrations idempotent; no panics; context timeouts present.