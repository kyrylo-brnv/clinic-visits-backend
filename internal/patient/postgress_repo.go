package patient

import (
	"context"
	"fmt"
	"strings"

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

func (r *PostgresRepository) FindPatients(
	ctx context.Context,
	request PatientSearchRequest,
) ([]Patient, error) {
	firstName := ""
	lastName := ""

	if request.Search != nil {
		firstName = escapeLike(request.Search.FirstName)
		lastName = escapeLike(request.Search.LastName)
	}

	var equalsID *string
	var notEqualsID *string

	if request.Filter != nil && !request.Filter.Id.IsEmpty() {
		equalsID = request.Filter.Id.Equals
		notEqualsID = request.Filter.Id.NotEquals
	}

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
                        AND ($1::uuid IS NULL OR id = $1::uuid)
                        AND ($2::uuid IS NULL OR id <> $2::uuid)
                        AND ($3 = '' OR first_name ILIKE '%' || $3 || '%')
                        AND ($4 = '' OR last_name ILIKE '%' || $4 || '%')
                ORDER BY created_at DESC
        `,
		equalsID,
		notEqualsID,
		firstName,
		lastName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query patients: %w", err)
	}
	defer rows.Close()

	patients := make([]Patient, 0)

	for rows.Next() {
		var row Patient

		if err := rows.Scan(
			&row.ID,
			&row.FirstName,
			&row.LastName,
			&row.DateOfBirth,
			&row.Gender,
			&row.IsDeleted,
		); err != nil {
			return nil, fmt.Errorf("failed to scan patient: %w", err)
		}

		patients = append(patients, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return patients, nil
}

func escapeLike(value string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	).Replace(value)
}
