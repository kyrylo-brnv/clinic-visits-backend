CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE visits
ADD CONSTRAINT visits_doctor_time_exclusion
EXCLUDE USING gist (
    doctor_id WITH =,
    tstzrange(visit_start_time, visit_end_time, '[)') WITH &&
);
