CREATE TABLE IF NOT EXISTS entities (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    layer       TEXT NOT NULL,
    status      TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    metadata    TEXT NOT NULL DEFAULT '{}',
    file_path   TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_entities_type   ON entities(type);
CREATE INDEX IF NOT EXISTS idx_entities_status ON entities(status);
CREATE INDEX IF NOT EXISTS idx_entities_layer  ON entities(layer);

CREATE TABLE IF NOT EXISTS relations (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    from_id  TEXT NOT NULL,
    to_id    TEXT NOT NULL,
    type     TEXT NOT NULL,
    layer    TEXT NOT NULL,
    weight   REAL NOT NULL DEFAULT 1.0,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT '',
    UNIQUE(from_id, to_id, type)
);

CREATE INDEX IF NOT EXISTS idx_relations_from ON relations(from_id);
CREATE INDEX IF NOT EXISTS idx_relations_to   ON relations(to_id);
CREATE INDEX IF NOT EXISTS idx_relations_type ON relations(type);

CREATE TABLE IF NOT EXISTS _meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS entities_fts USING fts5(
    id UNINDEXED, title
);
