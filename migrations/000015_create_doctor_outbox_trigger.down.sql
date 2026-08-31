DROP TRIGGER doctors_outbox_event_trigger ON doctors;

DROP FUNCTION enqueue_doctor_outbox_event();
