-- 0006_run_seed_and_rules.up.sql
-- Make TEXT the canonical seed, keep legacy BIGINT as seed_int, and add rules_version.
-- Idempotent and safe to run multiple times.

DO $$
BEGIN
    -- If runs.seed is BIGINT, rename it to seed_int (only if seed_int doesn't already exist)
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='runs' AND column_name='seed' AND data_type='bigint'
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_name='runs' AND column_name='seed_int'
        ) THEN
            EXECUTE 'ALTER TABLE runs RENAME COLUMN seed TO seed_int';
        END IF;
    END IF;

    -- Ensure seed TEXT exists
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='runs' AND column_name='seed'
    ) THEN
        EXECUTE 'ALTER TABLE runs ADD COLUMN seed TEXT';
    END IF;

    -- Ensure rules_version exists
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='runs' AND column_name='rules_version'
    ) THEN
        EXECUTE 'ALTER TABLE runs ADD COLUMN rules_version TEXT NOT NULL DEFAULT ''1.0.0''';
    END IF;

    -- Backfill seed TEXT from seed_int if seed is NULL
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='runs' AND column_name='seed_int'
    ) THEN
        EXECUTE 'UPDATE runs SET seed = seed_int::text WHERE seed IS NULL';
    END IF;
END$$;
