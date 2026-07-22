ALTER TABLE resources ADD COLUMN confidence_regenerability INTEGER NOT NULL DEFAULT 0 CHECK (confidence_regenerability BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN confidence_path_safety INTEGER NOT NULL DEFAULT 0 CHECK (confidence_path_safety BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN confidence_assessments TEXT NOT NULL DEFAULT '[]';
ALTER TABLE resources ADD COLUMN confidence_model_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE resources ADD COLUMN cleanup_disposition TEXT NOT NULL DEFAULT 'MANUAL_REVIEW';
ALTER TABLE resources ADD COLUMN risk_impact INTEGER NOT NULL DEFAULT 0 CHECK (risk_impact BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN risk_likelihood INTEGER NOT NULL DEFAULT 0 CHECK (risk_likelihood BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN risk_recoverability INTEGER NOT NULL DEFAULT 0 CHECK (risk_recoverability BETWEEN 0 AND 100);
ALTER TABLE resources ADD COLUMN risk_uncertainty INTEGER NOT NULL DEFAULT 0 CHECK (risk_uncertainty BETWEEN 0 AND 100);

UPDATE resources
SET confidence_regenerability = confidence_cleanup_safety,
    confidence_path_safety = confidence_cleanup_safety;

ALTER TABLE evidence ADD COLUMN claim TEXT;
ALTER TABLE evidence ADD COLUMN method TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence ADD COLUMN source_family TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence ADD COLUMN source_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence ADD COLUMN valid_until TEXT;
ALTER TABLE evidence ADD COLUMN polarity TEXT NOT NULL DEFAULT 'SUPPORTS';
