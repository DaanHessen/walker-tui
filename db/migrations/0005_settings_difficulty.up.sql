-- 0005_settings_difficulty.up.sql
-- Add difficulty column to settings with enum-like CHECK constraint.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='settings' AND column_name='difficulty'
    ) THEN
        EXECUTE 'ALTER TABLE settings ADD COLUMN difficulty TEXT NOT NULL DEFAULT ''standard''';
    END IF;
    -- Add constraint if missing
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname='chk_settings_difficulty'
    ) THEN
        EXECUTE 'ALTER TABLE settings ADD CONSTRAINT chk_settings_difficulty CHECK (difficulty IN (''easy'',''standard'',''hard''))';
    END IF;
END$$;
