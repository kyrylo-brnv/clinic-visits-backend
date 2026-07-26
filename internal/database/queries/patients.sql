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
        sqlc.arg('equals_id')::text = ''
        OR id = NULLIF(sqlc.arg('equals_id')::text, '')::uuid
    )
    AND (
        sqlc.arg('not_equals_id')::text = ''
        OR id <> NULLIF(sqlc.arg('not_equals_id')::text, '')::uuid
    )
    AND (
        sqlc.arg('first_name')::text = ''
        OR first_name ILIKE '%' || sqlc.arg('first_name')::text || '%'
    )
    AND (
        sqlc.arg('last_name')::text = ''
        OR last_name ILIKE '%' || sqlc.arg('last_name')::text || '%'
    )
ORDER BY created_at DESC;