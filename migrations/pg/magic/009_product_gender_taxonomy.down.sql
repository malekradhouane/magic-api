-- Revert the gender dimension. Seeded categories are managed by the startup
-- seeder and intentionally left untouched here.

DROP INDEX IF EXISTS idx_products_gender;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_gender_check;
ALTER TABLE products DROP COLUMN IF EXISTS gender;
