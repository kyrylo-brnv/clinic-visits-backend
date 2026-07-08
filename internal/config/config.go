package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPPort int
}

func LoadConfig() (*Config, error) {
	httpPortStr := os.Getenv("HTTP_PORT")
	if httpPortStr == "" {
		return nil, fmt.Errorf("HTTP_PORT is not configured")
	}

	httpPort, err := strconv.Atoi(httpPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP_PORT value: %w", err)
	}

	if httpPort <= 0 || httpPort > 65535 {
		return nil, fmt.Errorf("HTTP_PORT must be a valid port number between 1 and 65535")
	}

	return &Config{HTTPPort: httpPort}, nil
}
