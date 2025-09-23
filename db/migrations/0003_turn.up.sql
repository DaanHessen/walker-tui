-- 0003_turn.up.sql
-- Add turn outcome/update, archive, logs, settings tables.

CREATE TABLE IF NOT EXISTS updates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    scene_id UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    deltas JSONB NOT NULL,
    new_conditions TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS outcomes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    scene_id UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    outcome_md TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS archive_cards (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    survivor_id UUID NOT NULL REFERENCES survivors(id) ON DELETE CASCADE,
    world_day INT NOT NULL,
    region TEXT NOT NULL,
    cause_of_death TEXT NOT NULL,
    key_skills TEXT[] NOT NULL,
    notable_decisions TEXT[] NOT NULL,
    allies TEXT[] NOT NULL,
    final_inventory JSONB NOT NULL,
    card_md TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_archive_cards_run_day ON archive_cards(run_id, world_day);

CREATE TABLE IF NOT EXISTS master_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    survivor_id UUID NOT NULL REFERENCES survivors(id) ON DELETE CASCADE,
    choices_summary JSONB NOT NULL,
    narrative_recap_md TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS settings (
    run_id UUID PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE,
    scarcity BOOL NOT NULL DEFAULT TRUE,
    text_density TEXT NOT NULL DEFAULT 'standard',
    language TEXT NOT NULL DEFAULT 'en',
    narrator TEXT NOT NULL DEFAULT 'auto'
);
