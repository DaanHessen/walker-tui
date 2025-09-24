# Zero Point (Prototype)

## Quick Start
Requirements: Go 1.22+, Docker.

```bash
make db-up       # start postgres (docker)
make migrate-up  # apply migrations
make run         # launch game (AI off unless DEEPSEEK_API_KEY set)
```
Default DSN auto-applied if unset:
`postgres://dev:dev@localhost:5432/zeropoint?sslmode=disable`

## Controls
- 1-6 choose action
- Enter commit custom action (after typing)
- tab cycle views (scene/log/archive/settings)
- l logs view
- a archive view (Up/Down, Enter detail)
- s settings view
- t toggle scarcity (settings)
- d cycle text density (settings)
- n toggle narrator on/off (settings)
- e export run to `~/.zero-point/exports`
- pgup/pgdown scroll scene, home/end jump
- ? help toggle
- q / ctrl+c quit

## Implemented
- Postgres persistence (runs, survivors, scenes, choices, updates, outcomes, master logs, archive cards, settings)
- Enum constraints migration + idempotent column rename
- LAD heuristic + infection gating (risk softened pre-arrival) + debug toggle
- Deterministic RNG seed injection + pre-run world config (seed, scarcity, density)
- Transactional turn resolution + death -> archive + immediate respawn
- Multi-day loop (advances every 6 scenes placeholder)
- Choice generation influenced by scarcity, density, infection presence, basic skill hooks
- Custom action system with archetype mapping + gating (fatigue, critical needs, cooldown)
- AI narrator (DeepSeek Reasoner optional) with fallback minimal narrator and runtime toggle
- Timezone + local datetime in narrative state
- Archive cards + interactive archive browser
- Export full run narrative as markdown
- Settings view & dynamic choice regeneration
- Responsive TUI layout (top bar, sidebar, bottom bar, scrolling main pane)

## Planned / Not Yet
- Richer choice taxonomy (travel, crafting, social, shelter)
- Deeper skill progression & stat-driven modifiers
- Conditions system with mechanical effects
- Enhanced archive (AI epitaph, pivotal moments synthesis)
- Master log storyline stitching & timeline panel
- Streaming AI + improved retry/backoff
- Language switching / localization
- Unit & integration tests (LAD gating, first survivor distribution, custom action mapping, migration idempotence)
- Scrollback for timeline / dedicated log panel improvements
- Better world settings (difficulty presets, start region select)
- License text

## Environment
Set `DEEPSEEK_API_KEY` to enable AI narration.

## License
TBD

# Walker TUI

## Recent Additions
- Timeline view (press 'y') with scroll (PgUp/PgDn, arrows, Home/End, Esc to exit)
- Basic skill progression tied to choice archetypes (forage, rest, observe, organize, etc.)

## Controls (Extended)
- y: Open timeline view

### Difficulty Presets (Prototype)
Easy: +20% forage yield, lower fatigue drains.
Standard: baseline.
Hard: -25% forage yield, higher fatigue drains and moderate risks softened less.
