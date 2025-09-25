-- 0008_theme_and_tips.down.sql
-- Drop tips table and theme column/constraint.

DROP TABLE IF EXISTS tips;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname='chk_settings_theme'
    ) THEN
        EXECUTE 'ALTER TABLE settings DROP CONSTRAINT chk_settings_theme';
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name=''settings'' AND column_name=''theme''
    ) THEN
        EXECUTE 'ALTER TABLE settings DROP COLUMN theme';
    END IF;
END$$;
