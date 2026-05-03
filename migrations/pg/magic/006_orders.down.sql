DROP TRIGGER IF EXISTS orders_updated_at ON orders;
DROP FUNCTION IF EXISTS orders_updated_at_trigger();
DROP TABLE IF EXISTS order_items CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
