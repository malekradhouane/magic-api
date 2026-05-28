CREATE TABLE addresses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    label           VARCHAR(100) NOT NULL DEFAULT 'Domicile',
    first_name      VARCHAR(100) NOT NULL,
    last_name       VARCHAR(100) NOT NULL,
    phone           VARCHAR(50) NOT NULL,
    gouvernorat     VARCHAR(100) NOT NULL,
    address         TEXT NOT NULL,
    postal_code     VARCHAR(20),
    is_default      BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_addresses_user_id ON addresses(user_id);

-- At most one default address per user
CREATE UNIQUE INDEX idx_addresses_user_default
    ON addresses(user_id)
    WHERE is_default = true;
