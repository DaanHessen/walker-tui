-- 0011_profiles.up.sql
-- Introduce player profiles and attach runs to a profile for persistence.

CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO profiles (name)
SELECT 'main'
ON CONFLICT (name) DO NOTHING;

ALTER TABLE runs ADD COLUMN IF NOT EXISTS profile_id UUID;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS last_played_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE runs SET profile_id = (
    SELECT id FROM profiles WHERE name = 'main'
) WHERE profile_id IS NULL;

ALTER TABLE runs ALTER COLUMN profile_id SET NOT NULL;
ALTER TABLE runs ADD CONSTRAINT fk_runs_profile FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE;

ALTER TABLE runs ALTER COLUMN last_played_at DROP DEFAULT;
