-- 0011_profiles.down.sql
-- Drop player profiles linkage.

ALTER TABLE runs DROP CONSTRAINT IF EXISTS fk_runs_profile;
ALTER TABLE runs DROP COLUMN IF EXISTS profile_id;
ALTER TABLE runs DROP COLUMN IF EXISTS last_played_at;
DROP TABLE IF EXISTS profiles;
