-- Adds the "gender" audience dimension to products (separate from the
-- product-type category tree). The default category taxonomy itself is seeded
-- idempotently at application startup (see store SeedDefaultCategories), so it
-- can be kept up to date without new migrations.

ALTER TABLE products ADD COLUMN gender VARCHAR(20);
ALTER TABLE products
    ADD CONSTRAINT products_gender_check
    CHECK (gender IS NULL OR gender IN ('homme', 'femme', 'enfant', 'unisexe'));
CREATE INDEX idx_products_gender ON products (gender) WHERE gender IS NOT NULL;
