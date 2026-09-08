-- Three more bundled theme presets (issue #31) alongside the existing
-- "Dark Glass" default (006_seed_default_theme.sql) -- covers the
-- glass/solid x dark/light matrix so a fresh install has a real choice
-- beyond the one default. None of these set is_default; Dark Glass
-- stays it. Tokens must match web/src/shared/themes/presets.ts exactly
-- -- that file is the single source of truth these rows mirror (see
-- its own comment for why the values live in two places).
INSERT INTO themes (name, tokens, is_default) VALUES
(
    'Solid Dark',
    '{"background":{"type":"solid","value":"#14181f"},"cardBackground":"#1f2530","cardBorder":"#2c3341","textColor":"#e6e9ee","accentColor":"#4f9dff","fontFamily":"system-ui, sans-serif","fontSize":"16px","borderRadius":"12px","opacity":1,"blur":"0px"}',
    0
),
(
    'Glass Light',
    '{"background":{"type":"solid","value":"#f4f1ea"},"cardBackground":"rgba(255, 255, 255, 0.55)","cardBorder":"rgba(0, 0, 0, 0.08)","textColor":"#2a2320","accentColor":"#2f6fd6","fontFamily":"system-ui, sans-serif","fontSize":"16px","borderRadius":"12px","opacity":1,"blur":"16px"}',
    0
),
(
    'Solid Light',
    '{"background":{"type":"solid","value":"#f4f1ea"},"cardBackground":"#ffffff","cardBorder":"#e2ddd2","textColor":"#2a2320","accentColor":"#2f6fd6","fontFamily":"system-ui, sans-serif","fontSize":"16px","borderRadius":"12px","opacity":1,"blur":"0px"}',
    0
);
