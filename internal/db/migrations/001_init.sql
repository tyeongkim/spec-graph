CREATE TABLE IF NOT EXISTS entities (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    title        TEXT NOT NULL,
    description  TEXT,
    status       TEXT NOT NULL DEFAULT 'draft',
    metadata     TEXT NOT NULL DEFAULT '{}',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (type IN (
        'requirement', 'decision', 'phase', 'interface', 'state', 'test',
        'crosscut', 'question', 'assumption', 'criterion', 'risk'
    )),
    CHECK (status IN ('draft', 'active', 'deprecated', 'resolved', 'deleted'))
);

CREATE TABLE IF NOT EXISTS relations (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    from_id      TEXT NOT NULL REFERENCES entities(id),
    to_id        TEXT NOT NULL REFERENCES entities(id),
    type         TEXT NOT NULL,
    weight       REAL NOT NULL DEFAULT 1.0,
    metadata     TEXT NOT NULL DEFAULT '{}',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(from_id, to_id, type),
    CHECK (type IN (
        'implements', 'verifies', 'depends_on', 'constrained_by',
        'planned_in', 'delivered_in', 'triggers', 'answers', 'assumes',
        'has_criterion', 'mitigates', 'supersedes', 'conflicts_with', 'references'
    ))
);
