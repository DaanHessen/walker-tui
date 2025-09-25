-- 0007_events_schema.up.sql
-- Event instances tracking and narration cache.

CREATE TABLE IF NOT EXISTS event_instances (
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    survivor_id UUID NOT NULL REFERENCES survivors(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    world_day INT NOT NULL,
    scene_idx INT NOT NULL,
    cooldown_until_scene INT NOT NULL DEFAULT 0,
    arc_id TEXT,
    arc_step INT,
    once_fired BOOL NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, event_id, scene_idx)
);
CREATE INDEX IF NOT EXISTS idx_event_instances_run_survivor ON event_instances(run_id, survivor_id);

CREATE TABLE IF NOT EXISTS narration_cache (
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    state_hash BYTEA NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('scene','outcome')),
    text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, state_hash, kind)
);
