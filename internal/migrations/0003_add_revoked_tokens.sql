CREATE TABLE IF NOT EXISTS revoked_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- "jti" is the JWT ID claim. Revoking by ID rather than by token string
    -- means the record is small and unique, and one record invalidates every
    -- copy of a token.
    token_id    TEXT NOT NULL UNIQUE,
    user_id     UUID NOT NULL,
    revoked_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- An index on user_id supports "revoke every token for this user", which is
-- how a password change or a compromise response invalidates existing sessions.
CREATE INDEX IF NOT EXISTS idx_revoked_tokens_user_id ON revoked_tokens (user_id);
