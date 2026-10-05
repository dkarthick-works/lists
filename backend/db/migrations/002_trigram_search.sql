-- Serves both the substring (ILIKE '%q%') and fuzzy (q <% text) halves of
-- list autocomplete. Requires the pg_trgm extension from 000_schema.sql.
CREATE INDEX IF NOT EXISTS items_list_text_trgm_idx
    ON lists.items USING gin (text gin_trgm_ops)
    WHERE is_list;
