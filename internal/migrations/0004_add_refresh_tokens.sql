CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Deleting a user ends their sessions, so no refresh token can outlive
    -- the account it belongs to.
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Only the SHA-256 hash is stored. The plaintext is handed to the client
    -- once and cannot be recovered from here, so a leaked dump does not hand
    -- out working sessions.
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    -- Set when the token is rotated or explicitly revoked. Finding a non-null
    -- value on a presented token means it was replayed.
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Supports "revoke every token for this user", which is the response to a
-- replayed token and to a password change. The foreign key does not create an
-- index on its own.
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens (user_id);