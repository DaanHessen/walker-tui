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
- tab cycle views (scene/log/archive/settings)
- l logs view
- a archive view
- s settings view
- t toggle scarcity (inside settings)
- d cycle text density (inside settings)
- e export run to `~/.zero-point/exports`
- ? help toggle
- q / ctrl+c quit

## Implemented
- Postgres persistence (runs, survivors, scenes, choices, updates, outcomes, master logs, archive cards, settings)
- LAD heuristic + infection gating (risk softened pre-arrival)
- Deterministic RNG seed injection
- Transactional turn resolution + death -> archive + respawn
- Choice generation influenced by scarcity, density, infection presence
- AI narrator (DeepSeek Reasoner optional) with fallback structural narrator
- Archive cards (basic inventory & recent decisions)
- Export full run narrative as markdown
- Settings view & dynamic choice regeneration

## Planned / Not Yet
- World day advancement & multi-day loop
- Richer choice taxonomy (travel, crafting, social, shelter)
- Skill progression & effects, conditions system
- Enhanced archive (AI epitaph, pivotal moments)
- Master log storyline stitching
- Streaming AI + better backoff
- Language/narrator switching, localization
- Tests (unit/integration), CI pipeline, codegen for enums
- MySQL optional backend build tag
- UI layout improvements (panels, scrolling)

## Environment
Set `DEEPSEEK_API_KEY` to enable AI narration.

## License
TBD