ALTER TABLE visits
ADD CONSTRAINT visits_patient_time_exclusion
EXCLUDE USING gist (
    patient_id WITH =,
    tstzrange(visit_start_time, visit_end_time, '[)') WITH &&
);
