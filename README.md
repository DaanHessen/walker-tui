# Zero Point (Prototype)

## Quick Start
Requirements: Go 1.22+, Docker, DeepSeek API access.

```bash
make db-up       # start postgres (docker)
make migrate-up  # apply migrations
export DEEPSEEK_API_KEY=sk-... # required: director + narrator
make run         # launch game
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
- c cycle theme (settings)
- e export run to `~/.zero-point/exports`
- pgup/pgdown scroll scene, home/end jump
- y timeline view (PgUp/PgDn scroll, Esc to exit)
- ? help toggle
- q / ctrl+c quit

## Implemented
- Postgres persistence (runs, survivors, scenes, choices, updates, outcomes, master logs, archive cards, settings)
- Enum constraints migration + idempotent column rename
- LAD heuristic + infection gating (risk softened pre-arrival) + debug toggle
- Deterministic RNG seed injection + pre-run world config (seed, scarcity, density, theme)
- Transactional turn resolution + death -> archive + immediate respawn
- Multi-day loop (advances every 6 scenes placeholder)
- DeepSeek-only director + narrator driven by `docs/instructions.md`
- Loading overlay with rotating DB-backed tips while new runs bootstrap
- Choice generation influenced by scarcity, density, infection presence, basic skill hooks
- Custom action system with archetype mapping + gating (fatigue, critical needs, cooldown)
- Themed TUI (Catppuccin default + Dracula/Gruvbox/Solarized Dark)
- Timezone + local datetime in narrative state
- Archive cards + interactive archive browser
- Export full run narrative as markdown
- Settings view & dynamic choice regeneration, including theme preview
- Responsive TUI layout (top bar, sidebar, bottom bar, scrolling main pane)
- Timeline view with scrollback history

## Planned / Not Yet
- Richer choice taxonomy (travel, crafting, social, shelter)
- Deeper skill progression & stat-driven modifiers
- Conditions system with mechanical effects
- Enhanced archive (AI epitaph, pivotal moments synthesis)
- Streaming AI + improved retry/backoff
- Language switching / localization
- Unit & integration tests (LAD gating, first survivor distribution, custom action mapping, migration idempotence)
- Better world settings (difficulty presets, start region select)
- License text

## Environment
`DEEPSEEK_API_KEY` **must** be set; without it the TUI shows a blocking error screen.

## License
TBD

# Walker TUI

## Recent Additions
- Loading screen with rotating worldbuilding tips while DeepSeek warms up
- Theme selector (Catppuccin, Dracula, Gruvbox, Solarized Dark)
- Basic skill progression tied to choice archetypes (forage, rest, observe, organize, etc.)

### Difficulty Presets (Prototype)
Easy: +20% forage yield, lower fatigue drains.
Standard: baseline.
Hard: -25% forage yield, higher fatigue drains and moderate risks softened less.
