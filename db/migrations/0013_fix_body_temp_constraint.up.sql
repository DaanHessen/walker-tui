-- Update survivors body_temp constraint to include all temperature bands
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname='chk_survivors_body_temp'
    ) THEN
        ALTER TABLE survivors DROP CONSTRAINT chk_survivors_body_temp;
    END IF;
    ALTER TABLE survivors ADD CONSTRAINT chk_survivors_body_temp CHECK (body_temp IN ('arctic','cold','freezing','hot','mild','scorching','warm'));
END$$;
