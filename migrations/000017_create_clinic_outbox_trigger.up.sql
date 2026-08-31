CREATE FUNCTION enqueue_clinic_outbox_event()
RETURNS TRIGGER AS $$
DECLARE
    clinic_id UUID;
    clinic_event_type TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        clinic_id := OLD.id;
        clinic_event_type := 'clinic.deleted';
    ELSIF TG_OP = 'UPDATE' THEN
        clinic_id := NEW.id;
        clinic_event_type := 'clinic.updated';
    ELSE
        clinic_id := NEW.id;
        clinic_event_type := 'clinic.created';
    END IF;

    INSERT INTO outbox_events (
        aggregate_type,
        aggregate_id,
        event_type,
        payload
    )
    VALUES (
        'clinic',
        clinic_id,
        clinic_event_type,
        jsonb_build_object('id', clinic_id::text)
    );

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER clinics_outbox_event_trigger
AFTER INSERT OR UPDATE OR DELETE ON clinics
FOR EACH ROW
EXECUTE FUNCTION enqueue_clinic_outbox_event();
