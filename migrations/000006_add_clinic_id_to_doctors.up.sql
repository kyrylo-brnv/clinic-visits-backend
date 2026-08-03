ALTER TABLE doctors
ADD COLUMN clinic_id UUID REFERENCES clinics(id);

CREATE INDEX idx_doctors_clinic_id
ON doctors (clinic_id);