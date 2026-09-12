-- Multi-source calendar cards (issue #97). A join table rather than
-- widening cards.data_plugin_instance_id, so every existing widget
-- type keeps today's single-source behavior completely unchanged --
-- only a widget the frontend marks as multi-source-capable (currently
-- just calendar-agenda) ever writes rows here. cards.data_plugin_
-- instance_id is still kept in sync with the first id in the set (see
-- db.SetCardDataSources), so any code path that only reads the
-- singular column keeps working.
CREATE TABLE card_data_sources (
    card_id                  INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    data_plugin_instance_id  INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    PRIMARY KEY (card_id, data_plugin_instance_id)
);
CREATE INDEX idx_card_data_sources_card ON card_data_sources(card_id);
