-- 003_layers: Add layer column to entities and relations, update CHECK constraints
-- SQLite does not support ALTER COLUMN or modifying CHECK constraints,
-- so we recreate both tables with the updated schema.

-- Step 1: Recreate entities with layer column and updated type CHECK
CREATE TABLE entities_new (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    title        TEXT NOT NULL,
    description  TEXT,
    status       TEXT NOT NULL DEFAULT 'draft',
    metadata     TEXT NOT NULL DEFAULT '{}',
    layer        TEXT NOT NULL DEFAULT 'arch',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (type IN (
        'requirement', 'decision', 'phase', 'interface', 'state', 'test',
        'crosscut', 'question', 'assumption', 'criterion', 'risk', 'plan'
    )),
    CHECK (status IN ('draft', 'active', 'deprecated', 'resolved', 'deleted')),
    CHECK (layer IN ('arch', 'exec', 'mapping'))
);

INSERT INTO entities_new (id, type, title, description, status, metadata, layer, created_at, updated_at)
    SELECT id, type, title, description, status, metadata, 'arch', created_at, updated_at
    FROM entities;

-- Backfill: phase entities belong to exec layer
UPDATE entities_new SET layer = 'exec' WHERE type = 'phase';

-- Step 2: Recreate relations with layer column and updated type CHECK
CREATE TABLE relations_new (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    from_id      TEXT NOT NULL REFERENCES entities(id),
    to_id        TEXT NOT NULL REFERENCES entities(id),
    type         TEXT NOT NULL,
    weight       REAL NOT NULL DEFAULT 1.0,
    metadata     TEXT NOT NULL DEFAULT '{}',
    layer        TEXT NOT NULL DEFAULT 'arch',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(from_id, to_id, type),
    CHECK (type IN (
        'implements', 'verifies', 'depends_on', 'constrained_by',
        'planned_in', 'delivered_in', 'triggers', 'answers', 'assumes',
        'has_criterion', 'mitigates', 'supersedes', 'conflicts_with', 'references',
        'belongs_to', 'precedes', 'blocks', 'covers', 'delivers'
    )),
    CHECK (layer IN ('arch', 'exec', 'mapping'))
);

INSERT INTO relations_new (id, from_id, to_id, type, weight, metadata, layer, created_at)
    SELECT id, from_id, to_id, type, weight, metadata, 'arch', created_at
    FROM relations;

-- Backfill: planning relations belong to mapping layer
UPDATE relations_new SET layer = 'mapping' WHERE type IN ('planned_in', 'delivered_in');

-- Step 3: Swap tables
DROP TABLE relations;
DROP TABLE entities;

ALTER TABLE entities_new RENAME TO entities;
ALTER TABLE relations_new RENAME TO relations;

-- Step 4: Create indexes on layer columns
CREATE INDEX idx_entities_layer ON entities(layer);
CREATE INDEX idx_relations_layer ON relations(layer);
