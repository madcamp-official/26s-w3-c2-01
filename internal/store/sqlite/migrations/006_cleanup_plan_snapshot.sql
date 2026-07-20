ALTER TABLE cleanup_items ADD COLUMN normalized_path TEXT NOT NULL DEFAULT '';
ALTER TABLE cleanup_items ADD COLUMN expected_type TEXT NOT NULL DEFAULT '';
ALTER TABLE cleanup_items ADD COLUMN expected_modified_at TEXT NOT NULL DEFAULT '';
ALTER TABLE cleanup_items ADD COLUMN confidence_at_planning INTEGER NOT NULL DEFAULT 0 CHECK (confidence_at_planning BETWEEN 0 AND 100);
ALTER TABLE cleanup_items ADD COLUMN owner_project_id TEXT;
ALTER TABLE cleanup_items ADD COLUMN scan_id TEXT NOT NULL DEFAULT '';
ALTER TABLE cleanup_items ADD COLUMN regeneration_command TEXT;

CREATE UNIQUE INDEX idx_cleanup_items_plan_resource
    ON cleanup_items(plan_id, resource_id);
