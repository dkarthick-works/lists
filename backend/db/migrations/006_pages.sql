-- A page is an entry that carries long-form text: `text` holds its short
-- title and `body` the full text. Pages live in lists and hold no entries.
ALTER TABLE lists.items ADD COLUMN IF NOT EXISTS body TEXT;
