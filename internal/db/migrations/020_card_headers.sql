-- Optional card header (issue #145): a label distinguishing cards that
-- would otherwise only differ by content (e.g. two Calendar Agenda
-- cards for different rooms, or several Countdown cards). All three
-- columns are nullable and default to NULL -- an absent/empty
-- header_text means no header renders at all, matching the existing
-- theme_override/data_plugin_instance_id nullable-means-off
-- convention, so this is purely additive for every card that doesn't
-- use it.
ALTER TABLE cards ADD COLUMN header_text TEXT;
ALTER TABLE cards ADD COLUMN header_valign TEXT;
ALTER TABLE cards ADD COLUMN header_halign TEXT;
