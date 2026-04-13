-- Source categories (Бизнес-клубы, Холодная база, Рефералы...)
CREATE TABLE source_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_source_categories_user_id ON source_categories(user_id);

-- Concrete sources within categories (БК Магнат, 2GIS, ...)
CREATE TABLE lead_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES source_categories(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_lead_sources_user_id ON lead_sources(user_id);
CREATE INDEX idx_lead_sources_category_id ON lead_sources(category_id);

-- Add source_id FK to prospects and leads
ALTER TABLE prospects ADD COLUMN source_id UUID REFERENCES lead_sources(id) ON DELETE SET NULL;
ALTER TABLE leads ADD COLUMN source_id UUID REFERENCES lead_sources(id) ON DELETE SET NULL;
