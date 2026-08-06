package patient

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

type patientDocumentSearcher interface {
	Search(context.Context, string, any) ([]json.RawMessage, error)
}

// ElasticsearchRepository reads the Elasticsearch patient search model.
type ElasticsearchRepository struct {
	client patientDocumentSearcher
}

func NewElasticsearchRepository(client patientDocumentSearcher) *ElasticsearchRepository {
	return &ElasticsearchRepository{client: client}
}

func (r *ElasticsearchRepository) FindPatients(ctx context.Context, request PatientSearchRequest) ([]Patient, error) {
	documents, err := r.client.Search(ctx, elasticsearch.PatientsIndexName, patientSearchQuery(request))
	if err != nil {
		return nil, fmt.Errorf("search patients in Elasticsearch: %w", err)
	}

	patients := make([]Patient, 0, len(documents))
	for documentIndex, document := range documents {
		var source elasticsearch.PatientDocument
		if err := json.Unmarshal(document, &source); err != nil {
			return nil, fmt.Errorf("decode patient search result %d from Elasticsearch: %w", documentIndex, err)
		}

		patients = append(patients, Patient{
			ID:          source.ID,
			FirstName:   source.FirstName,
			LastName:    source.LastName,
			DateOfBirth: source.DateOfBirth.Format(time.DateOnly),
			Gender:      source.Gender,
			IsDeleted:   source.IsDeleted != nil && *source.IsDeleted,
		})
	}

	return patients, nil
}

func patientSearchQuery(request PatientSearchRequest) map[string]any {
	filters := []any{
		map[string]any{"term": map[string]bool{"is_deleted": false}},
	}
	must := make([]any, 0, 2)
	mustNot := make([]any, 0, 1)

	if !request.Search.isEmpty() {
		if request.Search.FirstName != "" {
			must = append(must, substringQuery("first_name.keyword", request.Search.FirstName))
		}
		if request.Search.LastName != "" {
			must = append(must, substringQuery("last_name.keyword", request.Search.LastName))
		}
	}

	if !request.Filter.isEmpty() {
		if request.Filter.Id.HasEquals() {
			filters = append(filters, map[string]any{"term": map[string]string{"id": *request.Filter.Id.Equals}})
		}
		if request.Filter.Id.HasNotEquals() {
			mustNot = append(mustNot, map[string]any{"term": map[string]string{"id": *request.Filter.Id.NotEquals}})
		}
	}

	return map[string]any{
		"from": request.Pagination.Offset(),
		"size": request.Pagination.Limit(),
		"query": map[string]any{
			"bool": map[string]any{
				"filter":   filters,
				"must":     must,
				"must_not": mustNot,
			},
		},
		"sort": patientSort(request),
	}
}

func substringQuery(field, value string) map[string]any {
	return map[string]any{"wildcard": map[string]any{
		field: map[string]any{
			"value":            "*" + escapeWildcard(value) + "*",
			"case_insensitive": true,
		},
	}}
}

func escapeWildcard(value string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`*`, `\*`,
		`?`, `\?`,
	).Replace(value)
}

func patientSort(request PatientSearchRequest) []map[string]map[string]string {
	if request.Sort == nil {
		return []map[string]map[string]string{
			{"created_at": {"order": "desc"}},
			{"id": {"order": "asc"}},
		}
	}

	field := request.Sort.Field
	if field == "first_name" || field == "last_name" {
		field += ".keyword"
	}

	return []map[string]map[string]string{
		{field: {"order": string(request.Sort.Direction)}},
		{"created_at": {"order": "desc"}},
		{"id": {"order": "asc"}},
	}
}
