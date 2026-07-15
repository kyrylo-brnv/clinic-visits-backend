 INSERT INTO patients (
      first_name,
      last_name,
      date_of_birth,
      gender
  )
  SELECT
      (ARRAY[
          'John', 'Anna', 'Michael', 'Olena', 'James',
          'Maria', 'David', 'Sofia', 'Robert', 'Emma',
          'Daniel', 'Liam', 'Noah', 'Lucas', 'Mia',
          'Emily', 'Alexander', 'Charlotte', 'Henry', 'Amelia',
          'William', 'Isabella', 'Benjamin', 'Evelyn', 'George',
          'Harper', 'Theodore', 'Ella', 'Jack', 'Grace',
          'Oliver', 'Lily', 'Jacob', 'Chloe', 'Leo',
          'Aria', 'Matthew', 'Nora', 'Sebastian', 'Ava',
          'Ethan', 'Camila', 'Samuel', 'Luna', 'Joseph',
          'Mila', 'Carter', 'Layla', 'Owen', 'Zoe',
          'Nathan', 'Leah', 'Dylan', 'Hannah', 'Gabriel',
          'Victoria', 'Julian', 'Scarlett', 'Isaac', 'Penelope',
          'Anthony', 'Aurora', 'Caleb', 'Ellie', 'Christopher',
          'Stella', 'Andrew', 'Lucy', 'Joshua', 'Maya',
          'Ryan', 'Naomi', 'Thomas', 'Eliana', 'Charles',
          'Elena', 'Aaron', 'Valeria', 'Adrian', 'Alice',
          'Adam', 'Eva', 'Max', 'Clara', 'Mark',
          'Roman', 'Kateryna', 'Dmytro', 'Yuliia', 'Andrii',
          'Anastasiia', 'Bohdan', 'Viktoriia', 'Taras', 'Iryna',
          'Mykola', 'Natalia', 'Pavlo', 'Tetiana', 'Serhii'
      ])[1 + floor(random() * 100)::int],

      (ARRAY[
          'Smith', 'Doe', 'Brown', 'Koval', 'Miller',
          'Wilson', 'Bondar', 'Taylor', 'Anderson', 'Shevchenko',
          'Johnson', 'Williams', 'Jones', 'Garcia', 'Martinez',
          'Robinson', 'Clark', 'Rodriguez', 'Lewis', 'Lee',
          'Walker', 'Hall', 'Allen', 'Young', 'King',
          'Wright', 'Scott', 'Green', 'Baker', 'Adams',
          'Nelson', 'Hill', 'Campbell', 'Mitchell', 'Roberts',
          'Carter', 'Phillips', 'Evans', 'Turner', 'Torres',
          'Parker', 'Collins', 'Edwards', 'Stewart', 'Sanchez',
          'Morris', 'Rogers', 'Reed', 'Cook', 'Morgan',
          'Bell', 'Murphy', 'Bailey', 'Rivera', 'Cooper',
          'Richardson', 'Cox', 'Howard', 'Ward', 'Peterson',
          'Gray', 'Ramirez', 'James', 'Watson', 'Brooks',
          'Kelly', 'Sanders', 'Price', 'Bennett', 'Wood',
          'Barnes', 'Ross', 'Henderson', 'Coleman', 'Jenkins',
          'Perry', 'Powell', 'Long', 'Patterson', 'Hughes',
          'Flores', 'Washington', 'Butler', 'Simmons', 'Foster',
          'Gonzalez', 'Bryant', 'Alexander', 'Russell', 'Griffin',
          'Diaz', 'Hayes', 'Myers', 'Ford', 'Hamilton',
          'Graham', 'Sullivan', 'Wallace', 'Woods', 'Cole'
      ])[1 + floor(random() * 100)::int],

      DATE '1940-01-01' + floor(random() * 29220)::int,

      (ARRAY['Male', 'Female'])
          [1 + floor(random() * 2)::int]

  FROM generate_series(1, 5000);