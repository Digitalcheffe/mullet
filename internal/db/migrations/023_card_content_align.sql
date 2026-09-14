-- Issue #149: lets a card position its widget's own content within the
-- card instead of always stretching to fill it -- null on both columns
-- (the default for every existing card) renders identically to today.
ALTER TABLE cards ADD COLUMN content_halign TEXT;
ALTER TABLE cards ADD COLUMN content_valign TEXT;
