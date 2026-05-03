-- Product images
CREATE TABLE product_images (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    url         TEXT NOT NULL,
    alt         VARCHAR(255),
    position    INT NOT NULL DEFAULT 0,
    is_primary  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_product_images_product_id ON product_images (product_id, position);
CREATE INDEX idx_product_images_primary ON product_images (product_id) WHERE is_primary = TRUE;

-- Product sizes (with stock per size)
CREATE TABLE product_sizes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    size        VARCHAR(20) NOT NULL,
    stock       INT NOT NULL DEFAULT 0 CHECK (stock >= 0),
    position    INT NOT NULL DEFAULT 0,

    UNIQUE (product_id, size)
);

CREATE INDEX idx_product_sizes_product_id ON product_sizes (product_id);
CREATE INDEX idx_product_sizes_in_stock ON product_sizes (product_id) WHERE stock > 0;

-- Product colors (with stock per color)
CREATE TABLE product_colors (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    hex         VARCHAR(7) NOT NULL,
    stock       INT NOT NULL DEFAULT 0 CHECK (stock >= 0),
    position    INT NOT NULL DEFAULT 0,

    UNIQUE (product_id, name)
);

CREATE INDEX idx_product_colors_product_id ON product_colors (product_id);
CREATE INDEX idx_product_colors_in_stock ON product_colors (product_id) WHERE stock > 0;
