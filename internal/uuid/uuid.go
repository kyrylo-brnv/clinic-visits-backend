package uuid

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

func Parse(value string) (pgtype.UUID, error) {
	if len(value) != 36 ||
		value[8] != '-' ||
		value[13] != '-' ||
		value[18] != '-' ||
		value[23] != '-' {
		return pgtype.UUID{}, fmt.Errorf("invalid UUID format")
	}

	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid UUID: %w", err)
	}

	return id, nil
}

func IsValid(value string) bool {
	_, err := Parse(value)
	return err == nil
}

func IsValidOptional(value string) bool {
	return value == "" || IsValid(value)
}
