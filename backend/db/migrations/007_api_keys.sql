-- Personal API keys for non-browser clients. Only the SHA-256 hex of a key is
-- stored; the raw key is shown once, when it is created.
CREATE TABLE IF NOT EXISTS lists.api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    name         TEXT NOT NULL,
    key_hash     TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS api_keys_user_idx ON lists.api_keys (user_id);
