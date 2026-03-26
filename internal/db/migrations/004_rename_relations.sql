-- 004_rename_relations: Remove v0.5 compatibility shims planned_in/delivered_in
-- These are replaced by covers/delivers (inverted direction, mapping layer).
-- SQLite does not support ALTER CHECK, so we recreate the relations table.

-- Step 1: Rename legacy relation types to their replacements.
-- planned_in (arch→phase) becomes covers (phase→arch): swap from_id/to_id.
-- delivered_in (arch→phase) becomes delivers (phase→arch): swap from_id/to_id.
UPDATE relations SET from_id = to_id, to_id = from_id, type = 'covers'
    WHERE type = 'planned_in';
UPDATE relations SET from_id = to_id, to_id = from_id, type = 'delivers'
    WHERE type = 'delivered_in';

-- Step 2: Ensure renamed relations are in the mapping layer.
UPDATE relations SET layer = 'mapping'
    WHERE type IN ('covers', 'delivers') AND layer != 'mapping';

-- Step 3: Recreate relations table with updated CHECK constraint (17 types).
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
        'triggers', 'answers', 'assumes',
        'has_criterion', 'mitigates', 'supersedes', 'conflicts_with', 'references',
        'belongs_to', 'precedes', 'blocks', 'covers', 'delivers'
    )),
    CHECK (layer IN ('arch', 'exec', 'mapping'))
);

INSERT INTO relations_new (id, from_id, to_id, type, weight, metadata, layer, created_at)
    SELECT id, from_id, to_id, type, weight, metadata, layer, created_at
    FROM relations;

-- Step 4: Swap tables
DROP TABLE relations;
ALTER TABLE relations_new RENAME TO relations;

-- Step 5: Recreate indexes
CREATE INDEX idx_relations_layer ON relations(layer);
