-- 0008_updates_conditions.up.sql
-- Rename updates.new_conditions to conditions_added and add conditions_removed column.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='updates' AND column_name='new_conditions'
    ) THEN
        EXECUTE 'ALTER TABLE updates RENAME COLUMN new_conditions TO conditions_added';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='updates' AND column_name='conditions_added'
    ) THEN
        EXECUTE 'ALTER TABLE updates ADD COLUMN conditions_added TEXT[] NOT NULL DEFAULT ''{}''';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='updates' AND column_name='conditions_removed'
    ) THEN
        EXECUTE 'ALTER TABLE updates ADD COLUMN conditions_removed TEXT[] NOT NULL DEFAULT ''{}''';
    END IF;
END$$;
