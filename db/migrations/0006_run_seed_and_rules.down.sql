-- 0006_run_seed_and_rules.down.sql
-- Down migration: drop canonical TEXT seed and rules_version.
-- Keep seed_int (legacy) for compatibility.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='runs' AND column_name='seed'
    ) THEN
        EXECUTE 'ALTER TABLE runs DROP COLUMN seed';
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='runs' AND column_name='rules_version'
    ) THEN
        EXECUTE 'ALTER TABLE runs DROP COLUMN rules_version';
    END IF;
END$$;
