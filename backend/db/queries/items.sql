-- name: ListTopLevel :many
SELECT * FROM lists.items
WHERE user_id = $1 AND parent_id IS NULL
ORDER BY lower(text), created_at;

-- name: ListRecent :many
-- Pinned lists have their own section on the home page, so they are left out.
SELECT * FROM lists.items
WHERE user_id = $1 AND parent_id IS NULL AND pinned_at IS NULL
ORDER BY updated_at DESC
LIMIT 5;

-- name: AutocompleteLists :many
-- Lists at any depth whose title contains the query or is a near miss of it
-- (trigram word similarity, so small typos still match). Prefix matches rank
-- first, then the closest titles.
SELECT i.id, i.text, p.text AS parent_text
FROM lists.items i
LEFT JOIN lists.items p ON p.id = i.parent_id
WHERE i.user_id = $1
  AND i.is_list
  AND (i.text ILIKE '%' || sqlc.arg(query)::text || '%' OR sqlc.arg(query)::text <% i.text)
ORDER BY
    (i.text ILIKE sqlc.arg(query)::text || '%') DESC,
    word_similarity(sqlc.arg(query)::text, i.text) DESC,
    i.updated_at DESC
LIMIT 8;

-- name: GetItem :one
SELECT * FROM lists.items
WHERE id = $1 AND user_id = $2;

-- name: ListEntries :many
-- Open entries in their manual order, then completed ones, oldest completion first.
SELECT * FROM lists.items
WHERE parent_id = $1 AND user_id = $2
ORDER BY completed_at NULLS FIRST, position, created_at, id;

-- name: ListAncestors :many
WITH RECURSIVE chain AS (
    SELECT p.id, p.parent_id, p.text, 1 AS depth
    FROM lists.items c
    JOIN lists.items p ON p.id = c.parent_id
    WHERE c.id = $1 AND c.user_id = $2
    UNION ALL
    SELECT p.id, p.parent_id, p.text, chain.depth + 1
    FROM chain
    JOIN lists.items p ON p.id = chain.parent_id
)
SELECT id, text FROM chain
ORDER BY depth DESC;

-- name: CreateItem :one
-- New entries go to the end of their list. A non-null body makes it a page.
INSERT INTO lists.items (user_id, parent_id, text, is_list, body, position)
VALUES (
    $1, $2, $3, $4, sqlc.narg(body),
    COALESCE((SELECT max(s.position) + 1 FROM lists.items s WHERE s.parent_id = $2), 0)
)
RETURNING *;

-- name: ReorderEntries :exec
-- Positions the given entries of one list in the order of the id array.
UPDATE lists.items i
SET position = o.ord
FROM unnest(sqlc.arg(ids)::uuid[]) WITH ORDINALITY AS o(id, ord)
WHERE i.id = o.id AND i.parent_id = sqlc.arg(parent_id) AND i.user_id = sqlc.arg(user_id);

-- name: UpdateItemText :one
UPDATE lists.items
SET text = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteItem :execrows
DELETE FROM lists.items
WHERE id = $1 AND user_id = $2;

-- name: TouchWithAncestors :exec
WITH RECURSIVE chain AS (
    SELECT i.id, i.parent_id FROM lists.items i WHERE i.id = $1
    UNION ALL
    SELECT p.id, p.parent_id
    FROM chain
    JOIN lists.items p ON p.id = chain.parent_id
)
UPDATE lists.items SET updated_at = now()
WHERE id IN (SELECT id FROM chain);

-- name: MakeItemList :one
UPDATE lists.items
SET is_list = true, updated_at = now()
WHERE id = $1 AND user_id = $2 AND body IS NULL
RETURNING *;

-- name: SetPinned :one
-- Only top-level lists can be pinned. Re-pinning keeps the original pin time,
-- and pinning does not count as an edit.
UPDATE lists.items
SET pinned_at = CASE WHEN sqlc.arg(pinned)::boolean THEN COALESCE(pinned_at, now()) END
WHERE id = $1 AND user_id = $2 AND parent_id IS NULL AND is_list
RETURNING *;

-- name: CountPinned :one
-- Pinned lists other than the given one.
SELECT count(*) FROM lists.items
WHERE user_id = $1 AND pinned_at IS NOT NULL AND id <> $2;

-- name: SetCompleted :one
-- Only entries (items inside a list) can be completed. Completing again keeps
-- the original completion time.
UPDATE lists.items
SET completed_at = CASE WHEN sqlc.arg(completed)::boolean THEN COALESCE(completed_at, now()) END,
    updated_at = now()
WHERE id = $1 AND user_id = $2 AND parent_id IS NOT NULL
RETURNING *;

-- name: UpdatePageBody :one
UPDATE lists.items
SET body = sqlc.arg(body)::text, updated_at = now()
WHERE id = $1 AND user_id = $2 AND body IS NOT NULL
RETURNING *;

-- name: ReplaceTitle :exec
-- Swaps a placeholder title for a generated one, on the item and on the page
-- directly inside it (a list made from long text shares its title with its
-- page). Anything the user has renamed since no longer matches and is left alone.
UPDATE lists.items
SET text = sqlc.arg(title)
WHERE user_id = sqlc.arg(user_id)
  AND (id = sqlc.arg(id) OR (parent_id = sqlc.arg(id) AND body IS NOT NULL))
  AND text = sqlc.arg(placeholder);
