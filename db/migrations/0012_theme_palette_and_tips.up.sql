-- Extend theme palette options and seed additional loading tips.

ALTER TABLE settings DROP CONSTRAINT IF EXISTS chk_settings_theme;
ALTER TABLE settings ADD CONSTRAINT chk_settings_theme CHECK (theme IN (
    'catppuccin','dracula','gruvbox','solarized_dark','nord','tokyonight','everforest'
));

INSERT INTO tips (id, text)
SELECT uuid_generate_v4(), tip
FROM (VALUES
    ('Balance loud and quiet actions so the noise meter never spikes on consecutive turns.'),
    ('Leadership skill rises when you coordinate group choices or delegate to allies.'),
    ('Use logistics skill to reduce fatigue costs on travel-heavy routes.'),
    ('Restoration scenes are safest when morale is above 40 and fatigue below 60.'),
    ('Custom actions inherit scarcity modifiers-watch the context banner for cues.'),
    ('Keep supply outlook healthy by cycling between forage, barter, and diplomacy arcs.'),
    ('Electronics skill unlocks calmer outcomes in infrastructure or control-room events.'),
    ('Mountaineering prowess makes canyon and ridge events far less punishing.'),
    ('Sailing knowledge opens waterborne exit routes once harbors destabilize.'),
    ('Forensics skill helps decode past survivor traces, shifting future guidance.'),
    ('When leadership trust is high, risky group gambits downgrade one risk tier.'),
    ('Psychology skill mitigates morale crashes after catastrophic outcomes.'),
    ('Rotate inventory inspection after each haul to prevent spoilage penalties.'),
    ('Stealth profile climbs when you favor scout, observe, or quiet custom actions.'),
    ('Prep firebreaks before storms-weather swings can trigger cascading hazard events.'),
    ('Catastrophic fatigue plus low morale invites collapse; schedule purposeful rest.'),
    ('Researcher starts reward technical or medicine choices before Day 0 breaches.'),
    ('Signal strength meter hints at upcoming radio or relay opportunities. Boost it early.'),
    ('Cold exposure meters reset slowly. Pair warm shelter with hot meals to recover.'),
    ('Logistics specialists reduce time costs on convoy or supply-chain missions.'),
    ('Griefing the infected with loud distractions risks panic unless trust is stable.'),
    ('Archive cards summarize lessons-review before committing your next survivor.'),
    ('Heatwave weather increases thirst drains; carry extra water before noon scenes.'),
    ('Ration morale boosters; celebrations during lull scenes can swing future odds.'),
    ('Combined medicine and psychology skills unlock calmer outcome narratives.'),
    ('Switch themes anytime to find a palette that matches your lighting.'),
    ('Nighttime scouting with high stealth profile unlocks low-risk intel events.'),
    ('Cooking skill extends food days whenever you resolve forage or barter successfully.'),
    ('Quartermasters flourish when supply outlook stays positive across three scenes.'),
    ('Monsoon fronts often precede flood or landslide events-shore up shelter first.'),
    ('Everforest theme pairs well with calmer narrative density.'),
    ('Nord theme was tuned for crisp contrast in bright terminals.'),
    ('Tokyonight theme mirrors neon stress-great with dense output.'),
    ('Keep an eye on leadership trust before attempting community negotiations.'),
    ('Use export (E) to snapshot the current run’s prose for later review.'),
    ('Toggle scarcity when you want director choices to lean harsher or kinder.'),
    ('Supply convoys respond better when logistics and negotiation are above level 2.'),
    ('Firefighter backgrounds shrug off barricade fatigue penalties more often.'),
    ('Mountaineers mitigate avalanche outcomes in tundra and canyon locations.'),
    ('Psychology helps survivors bounce back from archive-triggered trauma events.')
) AS seeds(tip)
WHERE NOT EXISTS (SELECT 1 FROM tips t WHERE t.text = seeds.tip);
