package doctor

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
)

type PostgresRepository struct {
	queries *sqlc.Queries
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{queries: sqlc.New(pool)}
}

func (r *PostgresRepository) FindDoctors(ctx context.Context, request DoctorSearchRequest) ([]Doctor, error) {
	params := sqlc.FindDoctorsParams{}

	if request.Filter != nil {
		params.DoctorID = request.Filter.DoctorID
		params.ClinicID = request.Filter.ClinicID
		params.VisitID = request.Filter.VisitID
	}

	rows, err := r.queries.FindDoctors(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find doctors: %w", err)
	}

	doctors := make([]Doctor, 0, len(rows))

	for _, row := range rows {
		doctors = append(doctors, Doctor{
			ID:          row.ID,
			SpecialtyID: row.SpecialtyID,
			ClinicID:    row.ClinicID,
			FullName:    row.FullName,
		})
	}

	return doctors, nil
}
