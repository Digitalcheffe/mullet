-- Issue #154: a card header (issue #145) never had a real use for
-- vertical alignment -- it always renders in its own reserved strip at
-- the top of the card now, never as a middle/bottom overlay. Only
-- header_halign (left/center/right) remains configurable.
ALTER TABLE cards DROP COLUMN header_valign;
