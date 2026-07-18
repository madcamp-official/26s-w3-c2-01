ALTER TABLE resources
ADD COLUMN size_known INTEGER NOT NULL DEFAULT 0 CHECK (size_known IN (0, 1));
