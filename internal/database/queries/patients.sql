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
ORDER BY
    CASE
        WHEN sqlc.arg('sort_field')::text = 'first_name'
            AND sqlc.arg('sort_direction')::text = 'asc'
        THEN first_name
    END ASC,
    CASE
        WHEN sqlc.arg('sort_field')::text = 'first_name'
            AND sqlc.arg('sort_direction')::text = 'desc'
        THEN first_name
    END DESC,
    CASE
        WHEN sqlc.arg('sort_field')::text = 'last_name'
            AND sqlc.arg('sort_direction')::text = 'asc'
        THEN last_name
    END ASC,
    CASE
        WHEN sqlc.arg('sort_field')::text = 'last_name'
            AND sqlc.arg('sort_direction')::text = 'desc'
        THEN last_name
    END DESC,
    CASE
        WHEN sqlc.arg('sort_field')::text = 'date_of_birth'
            AND sqlc.arg('sort_direction')::text = 'asc'
        THEN date_of_birth
    END ASC,
    CASE
        WHEN sqlc.arg('sort_field')::text = 'date_of_birth'
            AND sqlc.arg('sort_direction')::text = 'desc'
        THEN date_of_birth
    END DESC,
    CASE
        WHEN sqlc.arg('sort_field')::text = 'created_at'
            AND sqlc.arg('sort_direction')::text = 'asc'
        THEN created_at
    END ASC,
    CASE
        WHEN sqlc.arg('sort_field')::text = 'created_at'
            AND sqlc.arg('sort_direction')::text = 'desc'
        THEN created_at
    END DESC,
    created_at DESC,
    id ASC;