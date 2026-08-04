WITH seed_doctor_names AS (
    SELECT
        (ARRAY[
            'Amelia', 'Andrii', 'Anna', 'Bohdan', 'Danylo',
            'Daria', 'David', 'Emma', 'Iryna', 'James',
            'Kateryna', 'Liam', 'Maria', 'Michael', 'Natalia',
            'Olena', 'Oleksandr', 'Sofia', 'Taras', 'Yuliia'
        ])[1 + ((series_number - 1) % 20)] || ' ' ||
        (ARRAY[
            'Bondar', 'Brown', 'Koval', 'Melnyk', 'Miller',
            'Petrenko', 'Shevchenko', 'Smith', 'Taylor', 'Wilson'
        ])[1 + (((series_number - 1) / 10) % 10)] AS full_name
    FROM generate_series(1, 100) AS doctor_series(series_number)
)
DELETE FROM doctors AS doctor
USING specialties AS specialty, clinics AS clinic
WHERE doctor.specialty_id = specialty.id
    AND doctor.clinic_id = clinic.id
    AND doctor.full_name IN (
        SELECT full_name
        FROM seed_doctor_names
    )
    AND specialty.name IN (
        'Family Medicine',
        'Internal Medicine',
        'Pediatrics',
        'Cardiology',
        'Dermatology',
        'Neurology',
        'Orthopedics',
        'Ophthalmology',
        'Otolaryngology',
        'Psychiatry'
    )
    AND clinic.name IN (
        'Central Medical Clinic',
        'Podil Family Clinic',
        'Obolon Health Center',
        'Pechersk Medical Center',
        'Left Bank Clinic'
    );

DELETE FROM clinics
WHERE (name, address, time_zone) IN (
    VALUES
        (
            'Central Medical Clinic',
            '12 Khreshchatyk Street, Kyiv',
            'Europe/Kyiv'
        ),
        (
            'Podil Family Clinic',
            '24 Nyzhnii Val Street, Kyiv',
            'Europe/Kyiv'
        ),
        (
            'Obolon Health Center',
            '8 Obolonska Embankment, Kyiv',
            'Europe/Kyiv'
        ),
        (
            'Pechersk Medical Center',
            '15 Lesi Ukrainky Boulevard, Kyiv',
            'Europe/Kyiv'
        ),
        (
            'Left Bank Clinic',
            '7 Raisy Okipnoi Street, Kyiv',
            'Europe/Kyiv'
        )
);

DELETE FROM specialties
WHERE name IN (
    'Family Medicine',
    'Internal Medicine',
    'Pediatrics',
    'Cardiology',
    'Dermatology',
    'Neurology',
    'Orthopedics',
    'Ophthalmology',
    'Otolaryngology',
    'Psychiatry'
);
