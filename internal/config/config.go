package config

type AppServerConfig struct {
	HTTPPort int
}

type DatabaseConfig struct {
	Host     string `json:"host" env:"DB_HOST"`
	Port     int    `json:"port" env:"DB_PORT"`
	User     string `json:"user" env:"DB_USER"`
	Password string `json:"password" env:"DB_PASSWORD"`
	Name     string `json:"name" env:"DB_NAME"`
	SSLMode  string `json:"ssl_mode" env:"DB_SSLMODE"`
}

func LoadAppServerConfig() (*AppServerConfig, error) {
	httpPort, err := requiredIntConfig("HTTP_PORT")
	if err != nil {
		return nil, err
	}

	return &AppServerConfig{HTTPPort: httpPort}, nil
}

func LoadDatabaseConfig() (*DatabaseConfig, error) {
	host, err := requiredConfig("DB_HOST")
	if err != nil {
		return nil, err
	}

	port, err := requiredIntConfig("DB_PORT")
	if err != nil {
		return nil, err
	}

	user, err := requiredConfig("DB_USER")
	if err != nil {
		return nil, err
	}

	password, err := requiredConfig("DB_PASSWORD")
	if err != nil {
		return nil, err
	}

	name, err := requiredConfig("DB_NAME")
	if err != nil {
		return nil, err
	}

	sslMode, err := requiredConfig("DB_SSLMODE")
	if err != nil {
		return nil, err
	}

	return &DatabaseConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Name:     name,
		SSLMode:  sslMode,
	}, nil
}
