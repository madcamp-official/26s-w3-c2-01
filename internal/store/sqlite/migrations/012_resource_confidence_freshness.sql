ALTER TABLE resources ADD COLUMN confidence_freshness INTEGER NOT NULL DEFAULT 0 CHECK (confidence_freshness BETWEEN 0 AND 100);

UPDATE resources SET confidence_freshness = confidence;
