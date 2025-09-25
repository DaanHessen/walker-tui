-- 0009_seed_int_nullable.up.sql
-- Ensure legacy runs.seed_int is nullable to avoid NOT NULL violations when only seed TEXT is written.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='runs' AND column_name='seed_int'
    ) THEN
        BEGIN
            EXECUTE 'ALTER TABLE runs ALTER COLUMN seed_int DROP NOT NULL';
        EXCEPTION WHEN others THEN
            -- already nullable or constrained differently
            NULL;
        END;
    END IF;
END$$;

