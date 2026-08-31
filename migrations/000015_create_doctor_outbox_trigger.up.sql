CREATE FUNCTION enqueue_doctor_outbox_event()
RETURNS TRIGGER AS $$
DECLARE
    doctor_id UUID;
    doctor_event_type TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        doctor_id := OLD.id;
        doctor_event_type := 'doctor.deleted';
    ELSIF TG_OP = 'UPDATE' THEN
        doctor_id := NEW.id;
        doctor_event_type := 'doctor.updated';
    ELSE
        doctor_id := NEW.id;
        doctor_event_type := 'doctor.created';
    END IF;

    INSERT INTO outbox_events (
        aggregate_type,
        aggregate_id,
        event_type,
        payload
    )
    VALUES (
        'doctor',
        doctor_id,
        doctor_event_type,
        jsonb_build_object('id', doctor_id::text)
    );

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER doctors_outbox_event_trigger
AFTER INSERT OR UPDATE OR DELETE ON doctors
FOR EACH ROW
EXECUTE FUNCTION enqueue_doctor_outbox_event();
