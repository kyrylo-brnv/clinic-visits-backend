-- name: GetVisitForElasticsearchSync :one
SELECT
    v.id::text AS id,
    v.doctor_id::text AS doctor_id,
    v.patient_id::text AS patient_id,
    v.clinic_id::text AS clinic_id,
    v.status,
    v.visit_start_time,
    v.visit_end_time,
    v.created_at,
    v.updated_at,
    d.specialty_id::text AS doctor_specialty_id,
    d.clinic_id::text AS doctor_clinic_id,
    d.full_name AS doctor_full_name,
    p.first_name AS patient_first_name,
    p.last_name AS patient_last_name,
    p.date_of_birth AS patient_date_of_birth,
    p.gender AS patient_gender,
    p.is_deleted AS patient_is_deleted,
    c.name AS clinic_name,
    c.address AS clinic_address,
    c.time_zone AS clinic_time_zone
FROM visits v
JOIN doctors d ON d.id = v.doctor_id
JOIN patients p ON p.id = v.patient_id
JOIN clinics c ON c.id = v.clinic_id
WHERE v.id = sqlc.arg('visit_id');

-- name: ListDoctorsForElasticsearchSync :many
SELECT
    id::text AS id,
    specialty_id::text AS specialty_id,
    clinic_id::text AS clinic_id,
    full_name,
    created_at,
    updated_at
FROM doctors
WHERE id = ANY(sqlc.arg('doctor_ids')::uuid[])
ORDER BY id;

-- name: ListPatientsForElasticsearchSync :many
SELECT
    id::text AS id,
    first_name,
    last_name,
    date_of_birth,
    gender,
    is_deleted,
    created_at,
    updated_at
FROM patients
WHERE id = ANY(sqlc.arg('patient_ids')::uuid[])
ORDER BY id;

-- name: ListClinicsForElasticsearchSync :many
SELECT
    id::text AS id,
    name,
    address,
    time_zone,
    created_at,
    updated_at
FROM clinics
WHERE id = ANY(sqlc.arg('clinic_ids')::uuid[])
ORDER BY id;

-- name: ListVisitSummariesForElasticsearchSync :many
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
WHERE doctor_id = ANY(sqlc.arg('doctor_ids')::uuid[])
   OR patient_id = ANY(sqlc.arg('patient_ids')::uuid[])
   OR clinic_id = ANY(sqlc.arg('clinic_ids')::uuid[])
ORDER BY visit_start_time, id;
