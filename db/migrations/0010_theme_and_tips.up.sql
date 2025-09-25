-- 0008_theme_and_tips.up.sql
-- Add theme column to settings and create gameplay tips table.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='settings' AND column_name='theme'
    ) THEN
        EXECUTE 'ALTER TABLE settings ADD COLUMN theme TEXT NOT NULL DEFAULT ''catppuccin''';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname='chk_settings_theme'
    ) THEN
        EXECUTE 'ALTER TABLE settings ADD CONSTRAINT chk_settings_theme CHECK (theme IN (''catppuccin'',''dracula'',''gruvbox'',''solarized_dark''))';
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS tips (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed an initial batch of tips (idempotent on text uniqueness).
INSERT INTO tips (id, text)
SELECT uuid_generate_v4(), tip
FROM (VALUES
    ('Rotate survivors between high exertion and recovery actions to avoid exhaustion penalties.'),
    ('Scout unfamiliar blocks before committing to noisy work like barricading or salvage.'),
    ('Morale dips increase the risk of panic failures; mix in organizing or reflective actions.'),
    ('Food days run out quickly when fatigue spikes. Guard sleep as carefully as supplies.'),
    ('Use calm pre-arrival days to map safe corridors and stockpile water before contamination.'),
    ('Navigation skill grows fastest when alternating scout and travel archetypes.'),
    ('Barricading while exhausted can trigger injuries; rest or treat conditions first.'),
    ('Set aside medkits for fevers once the LAD hits—hospitals collapse almost immediately.'),
    ('Custom actions obey the same fatigue rules as menu choices—watch the meters.'),
    ('Archive cards remember notable decisions; review them to plan new survivor priorities.')
) AS seeds(tip)
WHERE NOT EXISTS (
    SELECT 1 FROM tips t WHERE t.text = seeds.tip
);
