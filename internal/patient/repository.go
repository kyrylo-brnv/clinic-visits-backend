package patient

import "context"

type Repository interface {
	List(ctx context.Context) ([]Patient, error)
}
