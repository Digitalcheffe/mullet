-- Screen-level font override (issue #87) -- mirrors cards.theme_override
-- (004_displays.sql) so a screen can pick its own font without a whole
-- new theme. Nullable, no default: absent means "use the display's own
-- theme," same convention as the card-level column.
ALTER TABLE screens ADD COLUMN theme_override TEXT;
