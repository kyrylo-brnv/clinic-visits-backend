WITH inserted_specialties AS (
    INSERT INTO specialties (name)
    VALUES
        ('Family Medicine'),
        ('Internal Medicine'),
        ('Pediatrics'),
        ('Cardiology'),
        ('Dermatology'),
        ('Neurology'),
        ('Orthopedics'),
        ('Ophthalmology'),
        ('Otolaryngology'),
        ('Psychiatry')
    RETURNING id, name
),
inserted_clinics AS (
    INSERT INTO clinics (name, address, time_zone)
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
    RETURNING id, name
)
INSERT INTO doctors (specialty_id, clinic_id, full_name)
SELECT
    specialty.id,
    clinic.id,
    (ARRAY[
        'Amelia', 'Andrii', 'Anna', 'Bohdan', 'Danylo',
        'Daria', 'David', 'Emma', 'Iryna', 'James',
        'Kateryna', 'Liam', 'Maria', 'Michael', 'Natalia',
        'Olena', 'Oleksandr', 'Sofia', 'Taras', 'Yuliia'
    ])[1 + ((series_number - 1) % 20)] || ' ' ||
    (ARRAY[
        'Bondar', 'Brown', 'Koval', 'Melnyk', 'Miller',
        'Petrenko', 'Shevchenko', 'Smith', 'Taylor', 'Wilson'
    ])[1 + (((series_number - 1) / 10) % 10)]
FROM generate_series(1, 100) AS doctor_series(series_number)
JOIN inserted_specialties AS specialty
    ON specialty.name = (ARRAY[
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
    ])[1 + ((series_number - 1) % 10)]
JOIN inserted_clinics AS clinic
    ON clinic.name = (ARRAY[
        'Central Medical Clinic',
        'Podil Family Clinic',
        'Obolon Health Center',
        'Pechersk Medical Center',
        'Left Bank Clinic'
    ])[1 + (((series_number - 1) / 10) % 5)];
