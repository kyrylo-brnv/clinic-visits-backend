-- name: FindPatients :many
SELECT
    id::text AS id,
    first_name,
    last_name,
    date_of_birth::text AS date_of_birth,
    gender,
    is_deleted
FROM patients
WHERE is_deleted = false
    AND (
        sqlc.narg('equals_id')::uuid IS NULL
        OR id = sqlc.narg('equals_id')::uuid
    )
    AND (
        sqlc.narg('not_equals_id')::uuid IS NULL
        OR id <> sqlc.narg('not_equals_id')::uuid
    )
    AND (
        sqlc.narg('first_name')::text IS NULL
        OR first_name ILIKE '%' || sqlc.narg('first_name')::text || '%'
    )
    AND (
        sqlc.narg('last_name')::text IS NULL
        OR last_name ILIKE '%' || sqlc.narg('last_name')::text || '%'
    )
ORDER BY created_at DESC;