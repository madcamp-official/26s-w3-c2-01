CREATE TABLE scan_issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_id TEXT NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    phase TEXT NOT NULL,
    adapter TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    operation TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL,
    message TEXT NOT NULL
);

CREATE INDEX idx_scan_issues_scan_id ON scan_issues(scan_id);
CREATE INDEX idx_scan_issues_scan_code ON scan_issues(scan_id, code);
CREATE INDEX idx_scan_issues_scan_severity ON scan_issues(scan_id, severity);
