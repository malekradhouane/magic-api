-- Create validation_tokens table for email verification
CREATE TABLE validation_tokens (
                           user_id    UUID NOT NULL,
                           token      VARCHAR(120) NOT NULL,
                           token_type VARCHAR(20) NOT NULL DEFAULT 'activation',
                           expired_at TIMESTAMPTZ  NOT NULL,
                           created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                           updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                           PRIMARY KEY (token),
                           FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create index on user_id for faster lookups
CREATE INDEX idx_validation_tokens_user_id ON validation_tokens(user_id);

-- Create index on expired_at for cleanup of expired tokens
CREATE INDEX idx_validation_tokens_expired_at ON validation_tokens(expired_at);

-- Create index on token_type for faster queries
CREATE INDEX idx_validation_tokens_token_type ON validation_tokens(token_type);

-- Create composite index on user_id and token_type for efficient lookups
CREATE INDEX idx_validation_tokens_user_type ON validation_tokens(user_id, token_type);
