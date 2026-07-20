ALTER TABLE transactions ADD COLUMN manifest_version INTEGER NOT NULL DEFAULT 1;

CREATE TABLE transaction_items (
    id TEXT PRIMARY KEY,
    transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    plan_item_id TEXT NOT NULL REFERENCES cleanup_items(id),
    resource_id TEXT NOT NULL REFERENCES resources(id),
    original_path TEXT NOT NULL,
    quarantine_path TEXT NOT NULL,
    manifest_path TEXT NOT NULL,
    expected_bytes INTEGER NOT NULL CHECK (expected_bytes >= 0),
    status TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_transaction_items_transaction_id ON transaction_items(transaction_id);
