-- 0004_constraints.down.sql
-- Revert enum constraints and column rename where applicable.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_survivors_location_type') THEN
        ALTER TABLE survivors DROP CONSTRAINT chk_survivors_location_type;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_survivors_group_type') THEN
        ALTER TABLE survivors DROP CONSTRAINT chk_survivors_group_type;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_survivors_body_temp') THEN
        ALTER TABLE survivors DROP CONSTRAINT chk_survivors_body_temp;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_choices_risk') THEN
        ALTER TABLE choices DROP CONSTRAINT chk_choices_risk;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_scenes_phase') THEN
        ALTER TABLE scenes DROP CONSTRAINT chk_scenes_phase;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name='master_logs' AND column_name='narrative_recap'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name='master_logs' AND column_name='narrative_recap_md'
    ) THEN
        ALTER TABLE master_logs RENAME COLUMN narrative_recap TO narrative_recap_md;
    END IF;
END$$;

DROP INDEX IF EXISTS idx_master_logs_run;
