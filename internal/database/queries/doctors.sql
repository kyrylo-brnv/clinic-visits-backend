-- name: FindDoctors :many
SELECT
    d.id::text AS id,
    d.specialty_id::text AS specialty_id,
    d.clinic_id::text AS clinic_id,
    d.full_name
FROM doctors d
WHERE
    (
        sqlc.arg('doctor_id')::text = ''
        OR d.id = NULLIF(sqlc.arg('doctor_id')::text, '')::uuid
    )
    AND (
        sqlc.arg('clinic_id')::text = ''
        OR d.clinic_id = NULLIF(sqlc.arg('clinic_id')::text, '')::uuid
    )
    AND (
        sqlc.arg('visit_id')::text = ''
        OR EXISTS (
            SELECT 1
            FROM visits v
            WHERE v.doctor_id = d.id
              AND v.id = NULLIF(sqlc.arg('visit_id')::text, '')::uuid
        )
    )
ORDER BY d.full_name, d.id;