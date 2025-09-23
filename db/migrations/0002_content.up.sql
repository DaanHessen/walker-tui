-- 0002_content.up.sql
-- Add narrative content tables.
CREATE TABLE IF NOT EXISTS scenes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    survivor_id UUID NOT NULL REFERENCES survivors(id) ON DELETE CASCADE,
    world_day INT NOT NULL,
    phase TEXT NOT NULL, -- scene | outcome
    lad INT NOT NULL,
    scene_md TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_scenes_run_day ON scenes(run_id, world_day);

CREATE TABLE IF NOT EXISTS choices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    scene_id UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    idx INT NOT NULL,
    label TEXT NOT NULL,
    cost JSONB NOT NULL,
    risk TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
