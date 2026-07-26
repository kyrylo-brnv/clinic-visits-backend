package patient

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
)

type PostgresRepository struct {
	queries *sqlc.Queries
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		queries: sqlc.New(pool),
	}
}

func (r *PostgresRepository) FindPatients(
	ctx context.Context,
	request PatientSearchRequest,
) ([]Patient, error) {
	params := sqlc.FindPatientsParams{}

	if !request.Search.isEmpty() {
		params.FirstName = escapeLike(request.Search.FirstName)
		params.LastName = escapeLike(request.Search.LastName)
	}

	if !request.Filter.isEmpty() {
		if request.Filter.Id.HasEquals() {
			params.EqualsID = *request.Filter.Id.Equals
		}

		if request.Filter.Id.HasNotEquals() {
			params.NotEqualsID = *request.Filter.Id.NotEquals
		}
	}

	if request.Sort != nil {
		params.SortField = request.Sort.Field
		params.SortDirection = string(request.Sort.Direction)
	}

	rows, err := r.queries.FindPatients(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("Failed to find patients: %w", err)
	}

	patients := make([]Patient, 0, len(rows))

	for _, row := range rows {
		patients = append(patients, Patient{
			ID:          row.ID,
			FirstName:   row.FirstName,
			LastName:    row.LastName,
			DateOfBirth: row.DateOfBirth,
			Gender:      row.Gender,
			IsDeleted:   row.IsDeleted.Bool,
		})
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
