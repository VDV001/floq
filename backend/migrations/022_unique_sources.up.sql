CREATE UNIQUE INDEX IF NOT EXISTS source_categories_user_name_unique ON source_categories (user_id, name);
CREATE UNIQUE INDEX IF NOT EXISTS lead_sources_category_name_unique ON lead_sources (category_id, name);
