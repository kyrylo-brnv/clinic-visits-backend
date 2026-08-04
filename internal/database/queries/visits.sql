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
    visit_start_time,
    visit_end_time,
    created_at,
    updated_at;
