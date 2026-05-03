-- Create users table
CREATE TABLE users (
                       id                    UUID PRIMARY KEY             DEFAULT gen_random_uuid(),
    -- Timestamps
                       created_at            TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
                       updated_at            TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
                       deleted_at            TIMESTAMPTZ,

    -- Authentication
                       email                 VARCHAR(255) UNIQUE NOT NULL,
                       password_hash         VARCHAR(255),
                       provider              VARCHAR(32)         NOT NULL DEFAULT 'email', -- 'email', 'google', 'github', etc.
                       provider_id           VARCHAR(255),                                 -- External provider ID
                       last_login_at         TIMESTAMPTZ,
                       role                  VARCHAR(255),


    -- User profile
                       username              VARCHAR(50) UNIQUE  NOT NULL,
                       first_name            VARCHAR(100),
                       last_name             VARCHAR(100),
                       avatar_url            TEXT,
                       phone_number          VARCHAR(50),
                       date_of_birth    VARCHAR(50),
                       gender                VARCHAR(20),
                       locale                VARCHAR(10),
    -- Account status
                       email_verified        BOOLEAN                      DEFAULT FALSE,
                       phone_verified        BOOLEAN                      DEFAULT FALSE,
                       is_active             BOOLEAN                      DEFAULT TRUE,
                       is_superuser          BOOLEAN                      DEFAULT FALSE,

    -- Security
                       mfa_enabled           BOOLEAN                      DEFAULT FALSE,
                       mfa_secret            VARCHAR(100),
                       last_password_change  TIMESTAMPTZ,
                       failed_login_attempts INT                          DEFAULT 0,
                       locked_until          TIMESTAMPTZ,

                       metadata         JSONB DEFAULT '{}'::jsonb,
                       full_text_search TSVECTOR
);

-- Indexes
CREATE INDEX idx_users_email ON users (LOWER(email));
CREATE INDEX idx_users_username_lower ON users (LOWER(username));
CREATE INDEX idx_users_created_at ON users (created_at);
CREATE INDEX idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_is_active ON users (is_active) WHERE is_active = TRUE;
CREATE INDEX idx_users_provider ON users (provider, provider_id) WHERE provider_id IS NOT NULL;

-- Full-text search index
-- The full_text_search column is already defined in the table
CREATE INDEX idx_users_fts ON users USING GIN(full_text_search);

-- Trigger to update the full-text search vector
CREATE
OR REPLACE FUNCTION update_users_search_vector() RETURNS TRIGGER AS $$
BEGIN
    NEW.full_text_search
=
        to_tsvector('english',
            COALESCE(NEW.username, '') || ' ' ||
            COALESCE(NEW.first_name, '') || ' ' ||
            COALESCE(NEW.last_name, '') || ' ' ||
            COALESCE(NEW.email, '')
        );
RETURN NEW;
END;
$$
LANGUAGE plpgsql;

CREATE TRIGGER users_search_vector_update
    BEFORE INSERT OR
UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_users_search_vector();

-- Update updated_at timestamp on row update
CREATE
OR REPLACE FUNCTION update_users_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at
= NOW();
RETURN NEW;
END;
$$
LANGUAGE plpgsql;

CREATE TRIGGER users_updated_at_trigger
    BEFORE UPDATE
    ON users
    FOR EACH ROW EXECUTE FUNCTION update_users_updated_at();