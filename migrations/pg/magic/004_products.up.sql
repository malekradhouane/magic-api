-- Products (prêt-à-porter)
CREATE TABLE products (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ,

    -- Identification
    slug             VARCHAR(255) UNIQUE NOT NULL,
    name             VARCHAR(255) NOT NULL,
    sku              VARCHAR(100) UNIQUE,

    -- Description
    description      TEXT,
    description_long TEXT,

    -- Pricing (in TND, 2 decimal places)
    price            NUMERIC(10, 2) NOT NULL CHECK (price >= 0),
    original_price   NUMERIC(10, 2) CHECK (original_price IS NULL OR original_price >= price),
    discount_percent INT GENERATED ALWAYS AS (
        CASE
            WHEN original_price IS NOT NULL AND original_price > 0
                THEN ROUND(((original_price - price) / original_price * 100)::numeric)::INT
            ELSE 0
        END
    ) STORED,
    currency         VARCHAR(3) NOT NULL DEFAULT 'TND',

    -- Categorization
    category_id      UUID REFERENCES categories(id) ON DELETE SET NULL,

    -- Status flags
    is_new           BOOLEAN NOT NULL DEFAULT FALSE,
    is_on_sale       BOOLEAN NOT NULL DEFAULT FALSE,
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    is_featured      BOOLEAN NOT NULL DEFAULT FALSE,

    -- Stats
    view_count       INT NOT NULL DEFAULT 0,
    sale_count       INT NOT NULL DEFAULT 0,

    -- SEO
    meta_title       VARCHAR(255),
    meta_description TEXT,

    -- Tags as array (for fashion: "soirée", "été", "élégant"...)
    tags             TEXT[] DEFAULT '{}',

    -- Free-form metadata
    metadata         JSONB DEFAULT '{}'::jsonb,

    -- Full text search
    full_text_search TSVECTOR
);

CREATE INDEX idx_products_slug ON products (slug);
CREATE INDEX idx_products_category_id ON products (category_id);
CREATE INDEX idx_products_active ON products (is_active) WHERE is_active = TRUE;
CREATE INDEX idx_products_new ON products (is_new) WHERE is_new = TRUE;
CREATE INDEX idx_products_sale ON products (is_on_sale) WHERE is_on_sale = TRUE;
CREATE INDEX idx_products_featured ON products (is_featured) WHERE is_featured = TRUE;
CREATE INDEX idx_products_price ON products (price);
CREATE INDEX idx_products_created_at ON products (created_at DESC);
CREATE INDEX idx_products_deleted_at ON products (deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_tags ON products USING GIN (tags);
CREATE INDEX idx_products_search ON products USING GIN (full_text_search);

-- Trigger: maintain full_text_search column
CREATE OR REPLACE FUNCTION products_search_vector_trigger() RETURNS trigger AS $$
BEGIN
    NEW.full_text_search :=
        setweight(to_tsvector('simple', coalesce(NEW.name, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(NEW.description, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(array_to_string(NEW.tags, ' '), '')), 'C');
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER products_search_vector_update
    BEFORE INSERT OR UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION products_search_vector_trigger();

-- Trigger: keep updated_at fresh
CREATE OR REPLACE FUNCTION products_updated_at_trigger() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := NOW();
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER products_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION products_updated_at_trigger();
