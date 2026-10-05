-- Manual ordering of entries within a list. Existing rows are numbered once,
-- in their old creation order; every statement is safe to re-run.
ALTER TABLE lists.items ADD COLUMN IF NOT EXISTS position INTEGER;

UPDATE lists.items i
SET position = n.rn
FROM (
    SELECT id, row_number() OVER (PARTITION BY parent_id ORDER BY created_at, id) - 1 AS rn
    FROM lists.items
) n
WHERE n.id = i.id AND i.position IS NULL;

ALTER TABLE lists.items ALTER COLUMN position SET DEFAULT 0;
ALTER TABLE lists.items ALTER COLUMN position SET NOT NULL;
