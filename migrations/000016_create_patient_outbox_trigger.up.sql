CREATE FUNCTION enqueue_patient_outbox_event()
RETURNS TRIGGER AS $$
DECLARE
    patient_id UUID;
    patient_event_type TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        patient_id := OLD.id;
        patient_event_type := 'patient.deleted';
    ELSIF TG_OP = 'UPDATE' THEN
        patient_id := NEW.id;
        patient_event_type := 'patient.updated';
    ELSE
        patient_id := NEW.id;
        patient_event_type := 'patient.created';
    END IF;

    INSERT INTO outbox_events (
        aggregate_type,
        aggregate_id,
        event_type,
        payload
    )
    VALUES (
        'patient',
        patient_id,
        patient_event_type,
        jsonb_build_object('id', patient_id::text)
    );

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER patients_outbox_event_trigger
AFTER INSERT OR UPDATE OR DELETE ON patients
FOR EACH ROW
EXECUTE FUNCTION enqueue_patient_outbox_event();
