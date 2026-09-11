ALTER TABLE release_notes
    ADD COLUMN audience VARCHAR(20) NOT NULL DEFAULT 'all' AFTER summary;
