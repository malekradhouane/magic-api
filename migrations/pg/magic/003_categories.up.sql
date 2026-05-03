-- Categories for products (abayas, caftans, djellabas, accessoires...)
CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,

    slug        VARCHAR(100) UNIQUE NOT NULL,
    name        VARCHAR(150) NOT NULL,
    description TEXT,
    image_url   TEXT,
    parent_id   UUID REFERENCES categories(id) ON DELETE SET NULL,
    position    INT NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,

    metadata    JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_categories_slug ON categories (slug);
CREATE INDEX idx_categories_parent_id ON categories (parent_id);
CREATE INDEX idx_categories_active ON categories (is_active) WHERE is_active = TRUE;
CREATE INDEX idx_categories_deleted_at ON categories (deleted_at) WHERE deleted_at IS NULL;
