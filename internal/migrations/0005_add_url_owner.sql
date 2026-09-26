-- Short links belong to the user who created them. Rows that predate
-- accounts keep a NULL owner; listing only ever returns rows that carry the
-- caller's ID, so ownerless rows are invisible rather than shared.
ALTER TABLE urls ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users (id) ON DELETE CASCADE;

-- Listing a user's own links is the second most common query after the
-- redirect lookup, so index the owner column.
CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls (user_id);