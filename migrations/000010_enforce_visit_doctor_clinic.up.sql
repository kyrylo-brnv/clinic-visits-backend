ALTER TABLE doctors
ADD CONSTRAINT doctors_id_clinic_id_key
UNIQUE (id, clinic_id);

ALTER TABLE visits
ADD CONSTRAINT visits_doctor_clinic_fkey
FOREIGN KEY (doctor_id, clinic_id)
REFERENCES doctors (id, clinic_id);
