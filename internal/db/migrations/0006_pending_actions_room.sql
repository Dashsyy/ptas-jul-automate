-- Widen pending_actions so a conversation can be anchored to a room without
-- needing a bill yet (e.g. /setname works even before any billing period
-- exists) — bill_id becomes nullable and a nullable room_id is added.
-- SQLite can't ALTER a column's NULL-ability directly, so recreate the
-- table; in-flight conversations are fine to drop across this upgrade.
CREATE TABLE pending_actions_new (
    chat_id    INTEGER PRIMARY KEY,
    kind       TEXT NOT NULL,
    bill_id    INTEGER REFERENCES bills(id),
    room_id    INTEGER REFERENCES rooms(id),
    step       TEXT NOT NULL,
    payload    TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL
);

DROP TABLE pending_actions;

ALTER TABLE pending_actions_new RENAME TO pending_actions;
