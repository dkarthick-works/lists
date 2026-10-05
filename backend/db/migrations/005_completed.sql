-- Entries can be marked completed. Completed entries sort after the rest but
-- keep their position, so un-completing one puts it back where it was.
ALTER TABLE lists.items ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;
