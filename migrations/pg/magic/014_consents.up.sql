-- Consents: stores proof of consent for marketing and contact-form submissions.
-- Required to answer complaints (e.g. Mailjet AUP 1d): keeps the date, the exact
-- consent text shown to the user, the source form, the IP address and user agent.

CREATE TABLE IF NOT EXISTS consents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),

    type         VARCHAR(32)  NOT NULL,
    source       VARCHAR(100) NOT NULL,

    email        VARCHAR(320) NOT NULL,
    name         VARCHAR(200),
    subject      VARCHAR(300),
    message      TEXT,

    consent      BOOLEAN      NOT NULL DEFAULT false,
    consent_text TEXT         NOT NULL,

    ip_address   VARCHAR(64),
    user_agent   TEXT,

    CONSTRAINT consents_type_check CHECK (type IN ('contact', 'newsletter'))
);

CREATE INDEX IF NOT EXISTS idx_consents_email ON consents (email);
CREATE INDEX IF NOT EXISTS idx_consents_type_created_at ON consents (type, created_at DESC);
