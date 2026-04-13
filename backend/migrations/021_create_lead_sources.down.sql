ALTER TABLE leads DROP COLUMN IF EXISTS source_id;
ALTER TABLE prospects DROP COLUMN IF EXISTS source_id;
DROP TABLE IF EXISTS lead_sources;
DROP TABLE IF EXISTS source_categories;
