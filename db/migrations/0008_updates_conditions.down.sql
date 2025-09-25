-- 0008_updates_conditions.down.sql
-- Revert updates table condition columns.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='updates' AND column_name='conditions_removed'
    ) THEN
        EXECUTE 'ALTER TABLE updates DROP COLUMN conditions_removed';
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='updates' AND column_name='conditions_added'
    ) THEN
        EXECUTE 'ALTER TABLE updates RENAME COLUMN conditions_added TO new_conditions';
    END IF;
END$$;
