-- 0004_constraints.up.sql
-- Add enum CHECK constraints and fix column mismatches; add helpful indexes.
-- Rename master_logs column to match code if it exists.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name='master_logs' AND column_name='narrative_recap_md'
    ) THEN
        EXECUTE 'ALTER TABLE master_logs RENAME COLUMN narrative_recap_md TO narrative_recap';
    END IF;
END$$;

-- Survivors enum constraints (Postgres CHECK). Added conditionally to avoid duplicate errors.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname='chk_survivors_location_type'
    ) THEN
        ALTER TABLE survivors ADD CONSTRAINT chk_survivors_location_type CHECK (location_type IN ('city','suburb','rural','forest','coast','mountain','desert'));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname='chk_survivors_group_type'
    ) THEN
        ALTER TABLE survivors ADD CONSTRAINT chk_survivors_group_type CHECK (group_type IN ('Solo','Duo','SmallGroup','Community'));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname='chk_survivors_body_temp'
    ) THEN
        ALTER TABLE survivors ADD CONSTRAINT chk_survivors_body_temp CHECK (body_temp IN ('freezing','cold','mild','warm','hot'));
    END IF;
END$$;

-- Choices risk enum constraint.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_choices_risk') THEN
        ALTER TABLE choices ADD CONSTRAINT chk_choices_risk CHECK (risk IN ('Low','Moderate','High'));
    END IF;
END$$;

-- Scenes phase constraint (scene|outcome|day) added if missing.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_scenes_phase') THEN
        ALTER TABLE scenes ADD CONSTRAINT chk_scenes_phase CHECK (phase IN ('scene','outcome','day'));
    END IF;
END$$;

-- Master logs index for run filtering.
CREATE INDEX IF NOT EXISTS idx_master_logs_run ON master_logs(run_id);

