-- name: CreateVisit :one
INSERT INTO visits (
    doctor_id,
    patient_id,
    clinic_id,
    visit_start_time,
    visit_end_time
)
VALUES (
    sqlc.arg('doctor_id'),
    sqlc.arg('patient_id'),
    sqlc.arg('clinic_id'),
    sqlc.arg('visit_start_time'),
    sqlc.arg('visit_end_time')
)
RETURNING
    id::text AS id,
    doctor_id::text AS doctor_id,
    patient_id::text AS patient_id,
    clinic_id::text AS clinic_id,
    status,
    visit_start_time,
    visit_end_time,
    created_at,
    updated_at;

-- name: ListVisits :many
SELECT
    id::text AS id,
    doctor_id::text AS doctor_id,
    patient_id::text AS patient_id,
    clinic_id::text AS clinic_id,
    status,
    visit_start_time,
    visit_end_time,
    created_at,
    updated_at
FROM visits
ORDER BY visit_start_time ASC, id ASC
LIMIT sqlc.arg('page_limit')::int
OFFSET sqlc.arg('page_offset')::bigint;

-- name: DeleteVisit :one
DELETE FROM visits
WHERE id = sqlc.arg('visit_id')
RETURNING
    id::text AS id,
    doctor_id::text AS doctor_id,
    patient_id::text AS patient_id,
    clinic_id::text AS clinic_id,
    status,
    visit_start_time,
    visit_end_time,
    created_at,
    updated_at;

-- name: UpdateVisit :one
UPDATE visits
SET
    doctor_id = COALESCE(sqlc.narg('doctor_id'), doctor_id),
    patient_id = COALESCE(sqlc.narg('patient_id'), patient_id),
    clinic_id = COALESCE(sqlc.narg('clinic_id'), clinic_id),
    visit_start_time = COALESCE(sqlc.narg('visit_start_time'), visit_start_time),
    visit_end_time = COALESCE(sqlc.narg('visit_end_time'), visit_end_time),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('visit_id')
RETURNING
    id::text AS id,
    doctor_id::text AS doctor_id,
    patient_id::text AS patient_id,
    clinic_id::text AS clinic_id,
    status,
    visit_start_time,
    visit_end_time,
    created_at,
    updated_at;

-- name: GetVisitStatusForUpdate :one
SELECT status
FROM visits
WHERE id = sqlc.arg('visit_id')
FOR UPDATE;
