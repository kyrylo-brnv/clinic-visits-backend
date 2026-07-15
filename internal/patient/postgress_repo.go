package patient

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

func (r *PostgresRepository) Search(
	ctx context.Context,
	request PatientSearchRequest,
) ([]Patient, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT 
			id::text, 
			first_name, 
			last_name, 
			date_of_birth::text,
			gender,
			is_deleted
		FROM patients 
		WHERE is_deleted = false
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query patients: %w", err)
	}
	defer rows.Close()

	patients := make([]Patient, 0)

	for rows.Next() {
		var row Patient

		err := rows.Scan(
			&row.ID,
			&row.FirstName,
			&row.LastName,
			&row.DateOfBirth,
			&row.Gender,
			&row.IsDeleted,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan patient: %w", err)
		}
		patients = append(patients, row)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", rows.Err())
	}

	return patients, nil
}
