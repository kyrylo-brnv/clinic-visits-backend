DROP TRIGGER patients_outbox_event_trigger ON patients;

DROP FUNCTION enqueue_patient_outbox_event();
