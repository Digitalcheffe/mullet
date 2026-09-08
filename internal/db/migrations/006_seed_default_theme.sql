-- Bundled default theme, so a fresh install has something usable to
-- assign to a display without building one first. Tokens must match
-- web/src/shared/themes/tokens.ts's defaultTheme exactly -- that file
-- is the single source of truth for the token shape and its values.
INSERT INTO themes (name, tokens, is_default)
VALUES (
    'Dark Glass',
    '{"background":{"type":"solid","value":"#0b0f14"},"cardBackground":"rgba(255, 255, 255, 0.06)","cardBorder":"rgba(255, 255, 255, 0.12)","textColor":"#e6e9ee","accentColor":"#4f9dff","fontFamily":"system-ui, sans-serif","fontSize":"16px","borderRadius":"12px","opacity":1,"blur":"16px"}',
    1
);
