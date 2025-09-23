-- 0001_init.up.sql
-- Core schema (Postgres only) minimal subset for early prototype.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    origin_site TEXT NOT NULL,
    seed BIGINT NOT NULL,
    current_day INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS survivors (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    age INT NOT NULL,
    background TEXT NOT NULL,
    region TEXT NOT NULL,
    location_type TEXT NOT NULL,
    group_type TEXT NOT NULL,
    group_size INT NOT NULL,
    traits TEXT[] NOT NULL,
    skills JSONB NOT NULL,
    stats JSONB NOT NULL,
    body_temp TEXT NOT NULL,
    conditions TEXT[] NOT NULL,
    meters JSONB NOT NULL,
    inventory JSONB NOT NULL,
    environment JSONB NOT NULL,
    alive BOOL NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_survivors_run_alive ON survivors(run_id, alive);
