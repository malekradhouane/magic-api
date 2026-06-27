-- Settings: singleton key-value store for application configuration.
-- Each row is a named setting group stored as a JSONB blob.
-- The "general" row always exists (seeded below).

CREATE TABLE IF NOT EXISTS settings (
    key         VARCHAR(100) PRIMARY KEY,
    value       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Seed the default "general" settings row.
INSERT INTO settings (key, value) VALUES
    ('general', '{
        "store_name": "Magic",
        "store_description": "",
        "contact_email": "",
        "contact_phone": "",
        "address": "",
        "city": "",
        "country": "Tunisie",
        "currency": "TND",
        "logo_url": "",
        "favicon_url": ""
    }'::jsonb),
    ('shipping', '{
        "free_shipping_threshold": 0,
        "default_shipping_cost": 8,
        "shipping_zones": []
    }'::jsonb),
    ('notifications', '{
        "order_email_enabled": true,
        "low_stock_threshold": 5,
        "notify_new_order": true,
        "notify_low_stock": true
    }'::jsonb),
    ('seo', '{
        "meta_title": "Magic",
        "meta_description": "",
        "og_image_url": "",
        "google_analytics_id": "",
        "facebook_pixel_id": ""
    }'::jsonb)
ON CONFLICT (key) DO NOTHING;
