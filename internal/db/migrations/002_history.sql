CREATE TABLE IF NOT EXISTS changesets (
    id         TEXT PRIMARY KEY,
    reason     TEXT NOT NULL DEFAULT '',
    actor      TEXT,
    source     TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS entity_history (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    changeset_id  TEXT NOT NULL REFERENCES changesets(id),
    entity_id     TEXT NOT NULL,
    action        TEXT NOT NULL CHECK (action IN ('create', 'update', 'deprecate', 'delete')),
    before_json   TEXT,
    after_json    TEXT,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS relation_history (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    changeset_id  TEXT NOT NULL REFERENCES changesets(id),
    relation_key  TEXT NOT NULL,
    action        TEXT NOT NULL CHECK (action IN ('create', 'delete')),
    before_json   TEXT,
    after_json    TEXT,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
