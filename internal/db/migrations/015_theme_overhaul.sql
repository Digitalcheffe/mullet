-- Theme system overhaul (issue #90): new ThemeTokens fields (cardStyle,
-- semantic colors, heading/size-scale typography) plus richer,
-- gradient-based bundled presets and a new brand-color theme.
--
-- The four existing bundled themes are upgraded to their new preset
-- values ONLY when their stored tokens still exactly match the original
-- pristine seed (006_seed_default_theme.sql / 013_bundled_themes.sql) --
-- an admin who already edited "Dark Glass" or any other bundled theme
-- keeps their own edits untouched. Tokens must match
-- web/src/shared/themes/tokens.ts and presets.ts exactly -- those files
-- are the single source of truth these rows mirror.

UPDATE themes SET tokens =
    '{"background":{"type":"gradient","value":"linear-gradient(160deg, #0b0f14 0%, #131a2b 60%, #1b2740 100%)"},"cardBackground":"rgba(255, 255, 255, 0.06)","cardBorder":"rgba(255, 255, 255, 0.12)","cardStyle":"glass","textColor":"#e6e9ee","accentColor":"#4f9dff","successColor":"#2ecc71","warningColor":"#f1c40f","errorColor":"#e74c3c","infoColor":"#4f9dff","fontFamily":"system-ui, sans-serif","fontSize":"16px","headingFontFamily":"system-ui, sans-serif","fontSizeSmall":"13px","fontSizeLarge":"28px","borderRadius":"12px","opacity":1,"blur":"16px"}'
    WHERE name = 'Dark Glass' AND tokens =
    '{"background":{"type":"solid","value":"#0b0f14"},"cardBackground":"rgba(255, 255, 255, 0.06)","cardBorder":"rgba(255, 255, 255, 0.12)","textColor":"#e6e9ee","accentColor":"#4f9dff","fontFamily":"system-ui, sans-serif","fontSize":"16px","borderRadius":"12px","opacity":1,"blur":"16px"}';

UPDATE themes SET tokens =
    '{"background":{"type":"gradient","value":"linear-gradient(160deg, #14181f 0%, #1c2230 100%)"},"cardBackground":"#1f2530","cardBorder":"#2c3341","cardStyle":"solid","textColor":"#e6e9ee","accentColor":"#4f9dff","successColor":"#2ecc71","warningColor":"#f1c40f","errorColor":"#e74c3c","infoColor":"#4f9dff","fontFamily":"system-ui, sans-serif","fontSize":"16px","headingFontFamily":"system-ui, sans-serif","fontSizeSmall":"13px","fontSizeLarge":"28px","borderRadius":"12px","opacity":1,"blur":"0px"}'
    WHERE name = 'Solid Dark' AND tokens =
    '{"background":{"type":"solid","value":"#14181f"},"cardBackground":"#1f2530","cardBorder":"#2c3341","textColor":"#e6e9ee","accentColor":"#4f9dff","fontFamily":"system-ui, sans-serif","fontSize":"16px","borderRadius":"12px","opacity":1,"blur":"0px"}';

UPDATE themes SET tokens =
    '{"background":{"type":"gradient","value":"linear-gradient(160deg, #f4f1ea 0%, #e8eef7 100%)"},"cardBackground":"rgba(255, 255, 255, 0.55)","cardBorder":"rgba(0, 0, 0, 0.08)","cardStyle":"glass","textColor":"#2a2320","accentColor":"#2f6fd6","successColor":"#3d9a6f","warningColor":"#c08a2e","errorColor":"#b5493c","infoColor":"#2f6fd6","fontFamily":"system-ui, sans-serif","fontSize":"16px","headingFontFamily":"system-ui, sans-serif","fontSizeSmall":"13px","fontSizeLarge":"28px","borderRadius":"12px","opacity":1,"blur":"16px"}'
    WHERE name = 'Glass Light' AND tokens =
    '{"background":{"type":"solid","value":"#f4f1ea"},"cardBackground":"rgba(255, 255, 255, 0.55)","cardBorder":"rgba(0, 0, 0, 0.08)","textColor":"#2a2320","accentColor":"#2f6fd6","fontFamily":"system-ui, sans-serif","fontSize":"16px","borderRadius":"12px","opacity":1,"blur":"16px"}';

UPDATE themes SET tokens =
    '{"background":{"type":"gradient","value":"linear-gradient(160deg, #f4f1ea 0%, #ece6da 100%)"},"cardBackground":"#ffffff","cardBorder":"#e2ddd2","cardStyle":"solid","textColor":"#2a2320","accentColor":"#2f6fd6","successColor":"#3d9a6f","warningColor":"#c08a2e","errorColor":"#b5493c","infoColor":"#2f6fd6","fontFamily":"system-ui, sans-serif","fontSize":"16px","headingFontFamily":"system-ui, sans-serif","fontSizeSmall":"13px","fontSizeLarge":"28px","borderRadius":"12px","opacity":1,"blur":"0px"}'
    WHERE name = 'Solid Light' AND tokens =
    '{"background":{"type":"solid","value":"#f4f1ea"},"cardBackground":"#ffffff","cardBorder":"#e2ddd2","textColor":"#2a2320","accentColor":"#2f6fd6","fontFamily":"system-ui, sans-serif","fontSize":"16px","borderRadius":"12px","opacity":1,"blur":"0px"}';

-- New bundled theme using Mullet's own logo palette (navy/cyan/red-pink)
-- -- purely additive, no existing row is affected.
INSERT INTO themes (name, tokens, is_default) VALUES
(
    'Mullet Brand',
    '{"background":{"type":"gradient","value":"linear-gradient(160deg, #011D4C 0%, #050b1a 100%)"},"cardBackground":"rgba(255, 255, 255, 0.07)","cardBorder":"rgba(1, 190, 252, 0.25)","cardStyle":"glass","textColor":"#eef6ff","accentColor":"#01BEFC","successColor":"#2ecc71","warningColor":"#f1c40f","errorColor":"#FB4460","infoColor":"#01BEFC","fontFamily":"system-ui, sans-serif","fontSize":"16px","headingFontFamily":"system-ui, sans-serif","fontSizeSmall":"13px","fontSizeLarge":"28px","borderRadius":"14px","opacity":1,"blur":"20px"}',
    0
);
