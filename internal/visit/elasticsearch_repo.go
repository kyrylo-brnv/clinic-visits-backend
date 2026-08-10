package visit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

const elasticsearchMaxResultWindow = 10_000

type visitDocumentSearcher interface {
	Search(context.Context, string, any) ([]json.RawMessage, error)
}

// ElasticsearchRepository reads the Elasticsearch visit list model.
type ElasticsearchRepository struct {
	client visitDocumentSearcher
}

func NewElasticsearchRepository(client visitDocumentSearcher) *ElasticsearchRepository {
	return &ElasticsearchRepository{client: client}
}

func (r *ElasticsearchRepository) ListVisits(
	ctx context.Context,
	request ListVisitsRequest,
) ([]Visit, error) {
	offset := request.Pagination.Offset()
	limit := request.Pagination.Limit()
	if offset <= elasticsearchMaxResultWindow-int64(limit) {
		return r.listVisits(ctx, visitListQuery(request))
	}

	var searchAfter []any
	for offset > 0 {
		batchSize := int32(elasticsearchMaxResultWindow)
		if offset < int64(batchSize) {
			batchSize = int32(offset)
		}

		visits, err := r.listVisits(ctx, visitSearchAfterQuery(batchSize, searchAfter))
		if err != nil {
			return nil, err
		}
		if len(visits) < int(batchSize) {
			return []Visit{}, nil
		}

		searchAfter = visitSearchAfterValues(visits[len(visits)-1])
		offset -= int64(len(visits))
	}

	return r.listVisits(ctx, visitSearchAfterQuery(limit, searchAfter))
}

func (r *ElasticsearchRepository) listVisits(ctx context.Context, query any) ([]Visit, error) {
	documents, err := r.client.Search(ctx, elasticsearch.VisitsIndexName, query)
	if err != nil {
		return nil, fmt.Errorf("search visits in Elasticsearch: %w", err)
	}

	visits := make([]Visit, 0, len(documents))
	for documentIndex, document := range documents {
		var source elasticsearch.VisitDocument
		if err := json.Unmarshal(document, &source); err != nil {
			return nil, fmt.Errorf("decode visit list result %d from Elasticsearch: %w", documentIndex, err)
		}

		visits = append(visits, Visit{
			ID:             source.ID,
			DoctorID:       source.DoctorID,
			PatientID:      source.PatientID,
			ClinicID:       source.ClinicID,
			Status:         VisitStatus(source.Status),
			VisitStartTime: source.VisitStartTime,
			VisitEndTime:   source.VisitEndTime,
			CreatedAt:      source.CreatedAt,
			UpdatedAt:      source.UpdatedAt,
		})
	}

	return visits, nil
}

func visitListQuery(request ListVisitsRequest) map[string]any {
	return map[string]any{
		"from": request.Pagination.Offset(),
		"size": request.Pagination.Limit(),
		"sort": []map[string]map[string]string{
			{"visit_start_time": {"order": "asc"}},
			{"id": {"order": "asc"}},
		},
	}
}

func visitSearchAfterQuery(size int32, searchAfter []any) map[string]any {
	query := map[string]any{
		"size": size,
		"sort": []map[string]map[string]string{
			{"visit_start_time": {"order": "asc"}},
			{"id": {"order": "asc"}},
		},
	}
	if len(searchAfter) > 0 {
		query["search_after"] = searchAfter
	}

	return query
}

func visitSearchAfterValues(visit Visit) []any {
	return []any{visit.VisitStartTime.Format(time.RFC3339Nano), visit.ID}
}
