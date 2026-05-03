-- Promo codes
CREATE TABLE promo_codes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    code            VARCHAR(50) UNIQUE NOT NULL,
    description     TEXT,

    -- Discount type: percentage or fixed amount
    discount_type   VARCHAR(20) NOT NULL
        CHECK (discount_type IN ('percentage', 'fixed')),
    discount_value  NUMERIC(10, 2) NOT NULL CHECK (discount_value >= 0),

    -- Constraints
    min_order_total NUMERIC(10, 2) DEFAULT 0,
    max_discount    NUMERIC(10, 2),

    -- Usage limits
    usage_limit     INT,             -- NULL = unlimited
    usage_count     INT NOT NULL DEFAULT 0,
    per_user_limit  INT DEFAULT 1,   -- NULL = unlimited per user

    -- Validity
    starts_at       TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX idx_promo_codes_code ON promo_codes (UPPER(code));
CREATE INDEX idx_promo_codes_active ON promo_codes (is_active, expires_at) WHERE is_active = TRUE;

-- Track promo usage per user/order
CREATE TABLE promo_usages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    promo_code_id   UUID NOT NULL REFERENCES promo_codes(id) ON DELETE CASCADE,
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    order_id        UUID REFERENCES orders(id) ON DELETE SET NULL,
    discount_amount NUMERIC(10, 2) NOT NULL
);

CREATE INDEX idx_promo_usages_promo_code_id ON promo_usages (promo_code_id);
CREATE INDEX idx_promo_usages_user_id ON promo_usages (user_id);
CREATE INDEX idx_promo_usages_order_id ON promo_usages (order_id);
