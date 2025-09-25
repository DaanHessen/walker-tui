# Zero Point

*An AI-directed survival game, where every choice could be your last...*

## What is this?

Zero Point drops you into a post-apocalyptic world where a synthetic virus has turned civilization into a living horror. But, this isn't your typical zombie game; it's an AI-powered narrative experience that creates unique, branching stories for each survivor you control. When your character dies (they will), the world keeps running, and you'll step into the shoes of someone else trying to navigate the same, unforgiving reality. Your `days after outbreak` won't change, your inventory and skills will. You can decide to join a group, build a civilization, or go at it alone. This world is yours to conquer.  

## Quick Start

**Requirements**: Go 1.22+, Docker, DeepSeek API key

```bash
# Clone the repo and navigate to it
git clone https://github.com/DaanHessen/walker-tui.git
cd walker-tui

# Get your database running
make db-up

# Apply database migrations
make migrate-up

# Set your DeepSeek API key (get one at https://platform.deepseek.com/api_keys)
export DEEPSEEK_API_KEY=sk-your-key-here

# Play!
make run
```

## Your first moments

When Zero Point starts, you'll see a character sheet for your randomly generated survivor—maybe Sarah, a 34-year-old nurse from the suburbs with a pragmatic streak and medical training. The world is at Day 0, but something is very wrong. News reports are scattered. People are on edge.

The AI narrator describes your immediate situation in vivid second-person prose, then presents you with 2-6 choices. Each choice shows its costs (time, fatigue, hunger, thirst, risk level) and reflects your character's capabilities. That medical training? It affects what options you see and how successful you'll be.

Choose wisely. The infection is spreading according to realistic geographic and transportation patterns. What starts as distant news reports will become very personal, very quickly.

## Key features

**🧠 AI-Driven Everything**: AI generates scenarios, narrates outcomes, and adapts to your playstyle. Every choice branch is contextually appropriate and narratively compelling.

**🌍 Persistent World Timeline**: When your character dies, you don't restart—you become someone else in the same evolving world. Previous characters' actions have consequences.

**📊 Realistic Infection Modeling**: The outbreak spreads from a specific ground-zero facility outward, following realistic transportation networks and response patterns. No "suddenly zombies everywhere" nonsense.

**⚙️ Deep Character Systems**: Skills matter. A technician sees different opportunities than a paramedic. Traits like "cautious" or "impulsive" shape available actions.

**🎨 Beautiful Terminal UI**: Seven curated themes (Catppuccin, Dracula, Gruvbox, Solarized Dark, Nord, Tokyo Night, Everforest) with centered menus, full-width scene layouts, and smooth interactions.

**🎲 Deterministic Chaos**: Every game uses a seed, so interesting worlds can be shared and replayed, but AI narration ensures the stories stay fresh.

**📚 Archive System**: Your fallen survivors become archive cards with AI-generated epitaphs and their most memorable decisions, browsable across sessions via persistent profiles.
**📝 Live Logs**: Hit `L` in-game to open the master log overlay. You can also tail logs from another terminal with `psql` (see Observability below).

## Advanced features

- **Custom Actions**: Type your own actions beyond the provided choices. The AI determines if they're feasible and what happens.
- **Export System**: Save complete narrative logs as markdown for sharing or posterity.
- **Timeline View**: Scroll back through the entire world's history across all your survivors.

## Controls

- **1-6**: Choose numbered actions
- **Enter**: Commit custom typed actions  
- **Tab**: Cycle between views (scene/log/archive/settings)
- **Y**: Timeline view with full world history
- **E**: Export current run to markdown
- **C**: Cycle themes in real-time
- **?**: Help and rules reference
- **Q**: Quit (with auto-save)

## Observability

- In-game: Press `L` to open the master log overlay while you play.
- Second terminal: export `DATABASE_URL` (or pass `--dsn`) and run:
  ```bash
  watch -n 3 'psql "$DATABASE_URL" -At -c "SELECT to_char(created_at, \'HH24:MI:SS\'), narrative_recap FROM master_logs ORDER BY created_at DESC LIMIT 8"'
  ```
  The command polls the latest log recaps every three seconds so you can follow the narrative stream in real time.

## Architecture

Zero Point is built in Go with a PostgreSQL backend and uses the Bubble Tea framework for its terminal UI. The AI integration uses DeepSeek's reasoner model for both strategic event planning and prose generation. State is carefully managed to ensure consistent world simulation while allowing for creative AI interpretation.

## License

```
She like to party like it's 1999
Ayy, girl, don't be playin' with my–
Now is not the time
It's closin' time, what you want? Nevermind
Hurry up, call the cut, don't be playin' 'round this time
```
