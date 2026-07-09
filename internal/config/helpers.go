package config

import (
	"fmt"
	"os"
	"strconv"
)

func requiredConfig(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is not configured", key)
	}
	return value, nil
}

func requiredIntConfig(key string) (int, error) {
	valueStr, err := requiredConfig(key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value: %w", key, err)
	}

	if value < 0 || value > 65535 {
		return 0, fmt.Errorf("%s value must be between 0 and 65535", key)
	}

	return value, nil
}
