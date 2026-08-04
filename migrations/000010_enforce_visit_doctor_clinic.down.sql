ALTER TABLE visits
DROP CONSTRAINT visits_doctor_clinic_fkey;

ALTER TABLE doctors
DROP CONSTRAINT doctors_id_clinic_id_key;
