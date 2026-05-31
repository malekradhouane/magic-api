-- Product variants: stock per (size, color) combination.
-- Replaces the independent product_sizes.stock / product_colors.stock model
-- so inventory is tracked per SKU (e.g. 20 t-shirts size M color Noir).
CREATE TABLE product_variants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    size        VARCHAR(20) NOT NULL,
    color       VARCHAR(100) NOT NULL,
    hex         VARCHAR(7) NOT NULL DEFAULT '#000000',
    stock       INT NOT NULL DEFAULT 0 CHECK (stock >= 0),
    position    INT NOT NULL DEFAULT 0,

    UNIQUE (product_id, size, color)
);

CREATE INDEX idx_product_variants_product_id ON product_variants (product_id);
CREATE INDEX idx_product_variants_in_stock ON product_variants (product_id) WHERE stock > 0;

-- Backfill: build a variant for every existing (size, color) pair of a product.
-- Stock is taken from the matching size row (the previous source of truth used
-- at checkout). Products with sizes but no colors get a single "Unique" color.
INSERT INTO product_variants (product_id, size, color, hex, stock, position)
SELECT
    ps.product_id,
    ps.size,
    COALESCE(pc.name, 'Unique')          AS color,
    COALESCE(pc.hex, '#000000')          AS hex,
    ps.stock,
    (ps.position * 100 + COALESCE(pc.position, 0)) AS position
FROM product_sizes ps
LEFT JOIN product_colors pc ON pc.product_id = ps.product_id
ON CONFLICT (product_id, size, color) DO NOTHING;
