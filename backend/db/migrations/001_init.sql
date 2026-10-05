-- A single tree: a top-level list is an item with no parent, an entry is an
-- item inside a list, and an entry with is_list = true is itself a list.
CREATE TABLE IF NOT EXISTS lists.items (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL,
    parent_id  UUID REFERENCES lists.items(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    is_list    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS items_parent_idx ON lists.items (parent_id, created_at);
CREATE INDEX IF NOT EXISTS items_user_recent_idx ON lists.items (user_id, updated_at DESC) WHERE is_list;
