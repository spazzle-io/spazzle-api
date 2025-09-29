-- name: AddWordsToServer :execresult
WITH next_index AS (
    SELECT COALESCE(MAX(word_idx), 0) + 1 AS start_index
    FROM words
    WHERE server_id = sqlc.arg(server_id)::uuid
)
INSERT INTO words (server_id, word, word_idx)
SELECT
    sqlc.arg(server_id)::uuid,
    w.word,
    next_index.start_index + ROW_NUMBER() OVER () - 1
FROM next_index
JOIN (
    SELECT unnest(sqlc.arg(words)::text[]) AS word
) AS w ON true
ON CONFLICT (server_id, word) DO NOTHING;

-- name: ListWords :many
SELECT
    id,
    server_id,
    word,
    added_at
FROM words
WHERE server_id = sqlc.arg(server_id)
AND (
    sqlc.narg(after_added_at)::timestamptz IS NULL
    OR added_at < sqlc.narg(after_added_at)
    OR (added_at = sqlc.narg(after_added_at) AND id < sqlc.narg(after_id))
) ORDER BY added_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: GetRandomWordsForServer :many
WITH bounds AS (
    SELECT MIN(word_idx) AS min_idx, MAX(word_idx) AS max_idx
    FROM words
    WHERE server_id = sqlc.arg(server_id)::uuid
),
random_indices AS (
    SELECT FLOOR(random() * (bounds.max_idx - bounds.min_idx + 1) + bounds.min_idx)::int AS idx
    FROM bounds, generate_series(1, sqlc.arg(n)::int * COALESCE(sqlc.narg(oversample_by_perc)::int, 10))
),
selected AS (
    SELECT id, server_id, word, added_at FROM (
        SELECT DISTINCT w.id, w.server_id, w.word, w.added_at
        FROM words w
        JOIN random_indices r ON w.word_idx = r.idx
        WHERE w.server_id = sqlc.arg(server_id)
    ) AS sub
    ORDER BY RANDOM()
    LIMIT sqlc.arg(n)
),
fallback AS (
    SELECT w.id, w.server_id, w.word, w.added_at
    FROM words w
    WHERE w.server_id = sqlc.arg(server_id)
    AND NOT EXISTS (
        SELECT 1 FROM selected s WHERE s.id = w.id
    )
    ORDER BY RANDOM()
    LIMIT GREATEST(sqlc.arg(n) - (SELECT COUNT(*) FROM selected), 0)
),
combined AS (
    SELECT * FROM selected
    UNION ALL
    SELECT * FROM fallback
)
SELECT * FROM combined
ORDER BY RANDOM();

-- name: RemoveWordsFromServer :execresult
DELETE FROM words
WHERE server_id = sqlc.arg(server_id)::uuid
AND word = ANY(sqlc.arg(words)::text[]);

-- name: RemoveAllWordsFromServer :execresult
DELETE FROM words
WHERE server_id = sqlc.arg(server_id)::uuid;
