-- name: ListTopLevel :many
SELECT * FROM lists.items
WHERE user_id = $1 AND parent_id IS NULL
ORDER BY lower(text), created_at;

-- name: ListRecent :many
SELECT * FROM lists.items
WHERE user_id = $1 AND parent_id IS NULL
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
SELECT * FROM lists.items
WHERE parent_id = $1 AND user_id = $2
ORDER BY created_at, id;

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
INSERT INTO lists.items (user_id, parent_id, text, is_list)
VALUES ($1, $2, $3, $4)
RETURNING *;

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
WHERE id = $1 AND user_id = $2
RETURNING *;
