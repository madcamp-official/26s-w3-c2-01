CREATE TABLE projects_v2 (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    project_type TEXT NOT NULL,
    root_path TEXT NOT NULL,
    normalized_root_path TEXT NOT NULL,
    manifest_path TEXT NOT NULL,
    normalized_manifest_path TEXT NOT NULL,
    drive TEXT NOT NULL,
    logical_size INTEGER NOT NULL DEFAULT 0 CHECK (logical_size >= 0),
    last_modified_at TEXT,
    last_observed_at TEXT NOT NULL,
    status TEXT NOT NULL,
    last_observed_scan_id TEXT NOT NULL REFERENCES scans(id),
    UNIQUE(project_type, normalized_manifest_path)
);

INSERT INTO projects_v2 (
    id, name, project_type, root_path, normalized_root_path,
    manifest_path, normalized_manifest_path, drive, logical_size,
    last_modified_at, last_observed_at, status, last_observed_scan_id
)
SELECT
    id, name, project_type, root_path, normalized_path,
    root_path, normalized_path, drive, 0,
    last_modified_at, last_observed_at, status, scan_id
FROM projects;

DROP TABLE projects;
ALTER TABLE projects_v2 RENAME TO projects;

CREATE INDEX idx_projects_scan_id ON projects(last_observed_scan_id);
CREATE INDEX idx_projects_type_status ON projects(project_type, status);
CREATE INDEX idx_projects_root ON projects(normalized_root_path);

CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    workspace_type TEXT NOT NULL,
    manifest_path TEXT NOT NULL,
    normalized_manifest_path TEXT NOT NULL,
    last_observed_at TEXT NOT NULL,
    last_observed_scan_id TEXT NOT NULL REFERENCES scans(id),
    UNIQUE(workspace_type, normalized_manifest_path)
);

CREATE INDEX idx_workspaces_scan_id ON workspaces(last_observed_scan_id);

CREATE TABLE workspace_projects (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id),
    PRIMARY KEY(workspace_id, project_id)
);

CREATE INDEX idx_workspace_projects_project_id ON workspace_projects(project_id);
