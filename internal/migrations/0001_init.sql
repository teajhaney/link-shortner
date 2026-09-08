CREATE TABLE IF NOT EXISTS urls (
    id          BIGSERIAL PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,
    long_url    TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    clicks      BIGINT NOT NULL DEFAULT 0
);

-- Speeds up the most common query: looking a code up on redirect.
CREATE INDEX IF NOT EXISTS idx_urls_code ON urls (code);
