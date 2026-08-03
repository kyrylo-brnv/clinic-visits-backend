CREATE TABLE visits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    doctor_id UUID NOT NULL REFERENCES doctors(id),
    patient_id UUID NOT NULL REFERENCES patients(id),
    clinic_id UUID NOT NULL REFERENCES clinics(id),
    visit_start_time TIMESTAMPTZ NOT NULL,
    visit_end_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT visits_valid_time_range
        CHECK (visit_end_time > visit_start_time)
);