-- Lists pinned to the top of the home page, in the order they were pinned.
ALTER TABLE lists.items ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMPTZ;
