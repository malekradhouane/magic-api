-- Orders (with guest checkout support: user_id nullable)
CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Order number (human-readable: AFR-YYYYMMDD-XXXX)
    order_number    VARCHAR(50) UNIQUE NOT NULL,

    -- Owner: nullable for guest checkout
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,

    -- Status: pending, confirmed, shipped, delivered, cancelled
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'confirmed', 'shipped', 'delivered', 'cancelled')),

    -- Pricing
    subtotal        NUMERIC(10, 2) NOT NULL CHECK (subtotal >= 0),
    shipping_fee    NUMERIC(10, 2) NOT NULL DEFAULT 0 CHECK (shipping_fee >= 0),
    discount_amount NUMERIC(10, 2) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    total_price     NUMERIC(10, 2) NOT NULL CHECK (total_price >= 0),
    currency        VARCHAR(3) NOT NULL DEFAULT 'TND',

    -- Promo code applied
    promo_code      VARCHAR(50),

    -- Payment
    payment_method  VARCHAR(20) NOT NULL DEFAULT 'cash'
        CHECK (payment_method IN ('cash', 'card', 'd17', 'paymee', 'konnect')),
    payment_status  VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (payment_status IN ('pending', 'paid', 'refunded', 'failed')),

    -- Shipping info (denormalized JSONB for guest checkout)
    -- { firstName, lastName, phone, gouvernorat, address, postalCode, notes }
    shipping_info   JSONB NOT NULL,

    -- Tracking
    tracking_number VARCHAR(100),
    shipped_at      TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    cancelled_at    TIMESTAMPTZ,

    -- Customer notes
    customer_notes  TEXT,

    -- Free-form metadata
    metadata        JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_orders_user_id ON orders (user_id);
CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_payment_status ON orders (payment_status);
CREATE INDEX idx_orders_order_number ON orders (order_number);
CREATE INDEX idx_orders_created_at ON orders (created_at DESC);
CREATE INDEX idx_orders_phone ON orders ((shipping_info->>'phone'));

-- Order items (line items)
CREATE TABLE order_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id      UUID REFERENCES products(id) ON DELETE SET NULL,

    -- Snapshot product data (in case product is deleted/updated later)
    product_name    VARCHAR(255) NOT NULL,
    product_image   TEXT,
    product_slug    VARCHAR(255),

    -- Variant snapshot
    size            VARCHAR(20),
    color           VARCHAR(100),

    -- Pricing snapshot at order time
    unit_price      NUMERIC(10, 2) NOT NULL CHECK (unit_price >= 0),
    quantity        INT NOT NULL CHECK (quantity > 0),
    line_total      NUMERIC(10, 2) NOT NULL CHECK (line_total >= 0)
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);
CREATE INDEX idx_order_items_product_id ON order_items (product_id);

-- Trigger: keep orders.updated_at fresh
CREATE OR REPLACE FUNCTION orders_updated_at_trigger() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := NOW();
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION orders_updated_at_trigger();
