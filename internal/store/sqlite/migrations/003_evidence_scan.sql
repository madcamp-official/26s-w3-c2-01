INSERT OR IGNORE INTO scans (
    id, started_at, finished_at, roots, file_count, error_count, status
)
SELECT
    'migration:003:legacy-evidence', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
    '[]', 0, 0, 'IMPORTED'
WHERE EXISTS (SELECT 1 FROM evidence);

CREATE TABLE evidence_v2 (
    id TEXT PRIMARY KEY,
    dependency_id TEXT NOT NULL REFERENCES dependencies(id) ON DELETE CASCADE,
    scan_id TEXT NOT NULL REFERENCES scans(id),
    evidence_type TEXT NOT NULL,
    source_path TEXT NOT NULL,
    property_name TEXT,
    raw_value TEXT,
    resolved_value TEXT,
    collected_at TEXT NOT NULL
);

INSERT INTO evidence_v2 (
    id, dependency_id, scan_id, evidence_type, source_path,
    property_name, raw_value, resolved_value, collected_at
)
SELECT
    id, dependency_id, 'migration:003:legacy-evidence', evidence_type,
    source_path, property_name, raw_value, resolved_value, collected_at
FROM evidence;

DROP TABLE evidence;
ALTER TABLE evidence_v2 RENAME TO evidence;

CREATE INDEX idx_evidence_dependency_id ON evidence(dependency_id);
CREATE INDEX idx_evidence_scan_id ON evidence(scan_id);
