ALTER TABLE resources ADD COLUMN confidence_classification INTEGER NOT NULL DEFAULT 0 CHECK (confidence_classification BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN confidence_ownership INTEGER NOT NULL DEFAULT 0 CHECK (confidence_ownership BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN confidence_dependency INTEGER NOT NULL DEFAULT 0 CHECK (confidence_dependency BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN confidence_cleanup_safety INTEGER NOT NULL DEFAULT 0 CHECK (confidence_cleanup_safety BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN confidence_scan_coverage INTEGER NOT NULL DEFAULT 0 CHECK (confidence_scan_coverage BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN risk_reasons TEXT NOT NULL DEFAULT '[]';

UPDATE resources SET
    confidence_classification = confidence,
    confidence_ownership = confidence,
    confidence_dependency = confidence,
    confidence_cleanup_safety = confidence,
    confidence_scan_coverage = confidence;
