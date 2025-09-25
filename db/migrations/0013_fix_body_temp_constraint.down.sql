-- Revert body_temp constraint update
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname='chk_survivors_body_temp'
    ) THEN
        ALTER TABLE survivors DROP CONSTRAINT chk_survivors_body_temp;
    END IF;
    ALTER TABLE survivors ADD CONSTRAINT chk_survivors_body_temp CHECK (body_temp IN ('freezing','cold','mild','warm','hot'));
END$$;
