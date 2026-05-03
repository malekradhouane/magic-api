DROP TRIGGER IF EXISTS products_updated_at ON products;
DROP TRIGGER IF EXISTS products_search_vector_update ON products;
DROP FUNCTION IF EXISTS products_updated_at_trigger();
DROP FUNCTION IF EXISTS products_search_vector_trigger();
DROP TABLE IF EXISTS products CASCADE;
