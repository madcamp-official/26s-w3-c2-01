CREATE TABLE scans (
    id TEXT PRIMARY KEY,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    roots TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0 CHECK (file_count >= 0),
    error_count INTEGER NOT NULL DEFAULT 0 CHECK (error_count >= 0),
    status TEXT NOT NULL
);

CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    root_path TEXT NOT NULL,
    normalized_path TEXT NOT NULL UNIQUE,
    drive TEXT NOT NULL,
    project_type TEXT NOT NULL,
    last_modified_at TEXT,
    last_observed_at TEXT NOT NULL,
    status TEXT NOT NULL,
    scan_id TEXT NOT NULL REFERENCES scans(id) ON DELETE CASCADE
);

CREATE INDEX idx_projects_scan_id ON projects(scan_id);
CREATE INDEX idx_projects_type_status ON projects(project_type, status);

CREATE TABLE resources (
    id TEXT PRIMARY KEY,
    resource_type TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT,
    path TEXT NOT NULL,
    normalized_path TEXT NOT NULL UNIQUE,
    logical_size INTEGER NOT NULL DEFAULT 0 CHECK (logical_size >= 0),
    reclaimable_size INTEGER NOT NULL DEFAULT 0 CHECK (reclaimable_size >= 0),
    regenerable INTEGER NOT NULL DEFAULT 0 CHECK (regenerable IN (0, 1)),
    system_managed INTEGER NOT NULL DEFAULT 0 CHECK (system_managed IN (0, 1)),
    last_modified_at TEXT,
    last_observed_at TEXT NOT NULL,
    risk TEXT NOT NULL,
    confidence INTEGER NOT NULL CHECK (confidence BETWEEN 0 AND 100)
);

CREATE INDEX idx_resources_type_risk ON resources(resource_type, risk);

CREATE TABLE dependencies (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    relation TEXT NOT NULL,
    confidence INTEGER NOT NULL CHECK (confidence BETWEEN 0 AND 100)
);

CREATE INDEX idx_dependencies_source ON dependencies(source_type, source_id);
CREATE INDEX idx_dependencies_target ON dependencies(target_type, target_id);

CREATE TABLE evidence (
    id TEXT PRIMARY KEY,
    dependency_id TEXT NOT NULL REFERENCES dependencies(id) ON DELETE CASCADE,
    evidence_type TEXT NOT NULL,
    source_path TEXT NOT NULL,
    property_name TEXT,
    raw_value TEXT,
    resolved_value TEXT,
    collected_at TEXT NOT NULL
);

CREATE INDEX idx_evidence_dependency_id ON evidence(dependency_id);

CREATE TABLE cleanup_plans (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    target_bytes INTEGER NOT NULL CHECK (target_bytes >= 0),
    selected_bytes INTEGER NOT NULL DEFAULT 0 CHECK (selected_bytes >= 0),
    status TEXT NOT NULL
);

CREATE TABLE cleanup_items (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL REFERENCES cleanup_plans(id) ON DELETE CASCADE,
    resource_id TEXT NOT NULL REFERENCES resources(id),
    expected_bytes INTEGER NOT NULL CHECK (expected_bytes >= 0),
    risk TEXT NOT NULL,
    action_type TEXT NOT NULL,
    reason TEXT NOT NULL
);

CREATE INDEX idx_cleanup_items_plan_id ON cleanup_items(plan_id);

CREATE TABLE transactions (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL REFERENCES cleanup_plans(id),
    started_at TEXT NOT NULL,
    finished_at TEXT,
    status TEXT NOT NULL
);

CREATE INDEX idx_transactions_plan_id ON transactions(plan_id);
