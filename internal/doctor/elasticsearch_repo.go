package doctor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

type doctorDocumentSearcher interface {
	Search(context.Context, string, any) ([]json.RawMessage, error)
}

// ElasticsearchRepository reads the Elasticsearch doctor search model.
type ElasticsearchRepository struct {
	client doctorDocumentSearcher
}

func NewElasticsearchRepository(client doctorDocumentSearcher) *ElasticsearchRepository {
	return &ElasticsearchRepository{client: client}
}

func (r *ElasticsearchRepository) FindDoctors(ctx context.Context, request DoctorSearchRequest) ([]Doctor, error) {
	documents, err := r.client.Search(ctx, elasticsearch.DoctorsIndexName, doctorSearchQuery(request))
	if err != nil {
		return nil, fmt.Errorf("search doctors in Elasticsearch: %w", err)
	}

	doctors := make([]Doctor, 0, len(documents))
	for documentIndex, document := range documents {
		var source elasticsearch.DoctorDocument
		if err := json.Unmarshal(document, &source); err != nil {
			return nil, fmt.Errorf("decode doctor search result %d from Elasticsearch: %w", documentIndex, err)
		}

		doctors = append(doctors, Doctor{
			ID:          source.ID,
			SpecialtyID: source.SpecialtyID,
			ClinicID:    source.ClinicID,
			FullName:    source.FullName,
		})
	}

	return doctors, nil
}

func doctorSearchQuery(request DoctorSearchRequest) map[string]any {
	filters := make([]any, 0, 3)
	if request.Filter != nil {
		if request.Filter.DoctorID != "" {
			filters = append(filters, map[string]any{
				"term": map[string]string{"id": request.Filter.DoctorID},
			})
		}
		if request.Filter.VisitID != "" {
			filters = append(filters, map[string]any{
				"nested": map[string]any{
					"path": "visits",
					"query": map[string]any{
						"term": map[string]string{"visits.id": request.Filter.VisitID},
					},
				},
			})
		}
		if request.Filter.ClinicID != "" {
			filters = append(filters, map[string]any{
				"term": map[string]string{"clinic_id": request.Filter.ClinicID},
			})
		}
	}

	return map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"filter": filters,
			},
		},
		"sort": []map[string]map[string]string{
			{"full_name.keyword": {"order": "asc"}},
			{"id": {"order": "asc"}},
		},
	}
}
