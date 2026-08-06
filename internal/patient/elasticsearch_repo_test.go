package patient

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
	"github.com/smithautotest/clinic-visits/internal/filter"
	"github.com/smithautotest/clinic-visits/internal/pagination"
	"github.com/smithautotest/clinic-visits/internal/sorting"
)

type fakePatientDocumentSearcher struct {
	indexName string
	query     any
	documents []json.RawMessage
	err       error
}

func (s *fakePatientDocumentSearcher) Search(_ context.Context, indexName string, query any) ([]json.RawMessage, error) {
	s.indexName = indexName
	s.query = query
	return s.documents, s.err
}

func TestElasticsearchRepositoryFindPatientsBuildsV1CompatibleQuery(t *testing.T) {
	paginationParams, err := pagination.Parse(url.Values{"page": {"3"}, "per_page": {"50"}})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	equalsID := "patient-1"
	notEqualsID := "patient-2"
	searcher := &fakePatientDocumentSearcher{
		documents: []json.RawMessage{
			json.RawMessage(`{"id":"patient-1","first_name":"Ann","last_name":"O'Neil","date_of_birth":"1990-01-01T00:00:00Z","gender":"Female","is_deleted":false}`),
		},
	}
	repository := NewElasticsearchRepository(searcher)

	patients, err := repository.FindPatients(t.Context(), PatientSearchRequest{
		Search:     &PatientSearch{FirstName: "Ann*", LastName: "O?Neil"},
		Filter:     &PatientFilter{Id: &filter.StringFilter{Equals: &equalsID, NotEquals: &notEqualsID}},
		Sort:       &sorting.Sort{Field: "first_name", Direction: sorting.Ascending},
		Pagination: paginationParams,
	})
	if err != nil {
		t.Fatalf("FindPatients() error = %v", err)
	}

	if searcher.indexName != elasticsearch.PatientsIndexName {
		t.Fatalf("Search() index = %q, want %q", searcher.indexName, elasticsearch.PatientsIndexName)
	}
	if !reflect.DeepEqual(searcher.query, map[string]any{
		"from": int64(100),
		"size": int32(50),
		"query": map[string]any{"bool": map[string]any{
			"filter": []any{
				map[string]any{"term": map[string]bool{"is_deleted": false}},
				map[string]any{"term": map[string]string{"id": "patient-1"}},
			},
			"must": []any{
				substringQuery("first_name.keyword", "Ann*"),
				substringQuery("last_name.keyword", "O?Neil"),
			},
			"must_not": []any{map[string]any{"term": map[string]string{"id": "patient-2"}}},
		}},
		"sort": []map[string]map[string]string{
			{"first_name.keyword": {"order": "asc"}},
			{"created_at": {"order": "desc"}},
			{"id": {"order": "asc"}},
		},
	}) {
		t.Fatalf("Search() query = %#v", searcher.query)
	}

	wantPatients := []Patient{{
		ID:          "patient-1",
		FirstName:   "Ann",
		LastName:    "O'Neil",
		DateOfBirth: "1990-01-01",
		Gender:      "Female",
	}}
	if !reflect.DeepEqual(patients, wantPatients) {
		t.Fatalf("FindPatients() = %#v, want %#v", patients, wantPatients)
	}
}

func TestElasticsearchRepositoryFindPatientsReportsSearchAndDecodeErrors(t *testing.T) {
	searcher := &fakePatientDocumentSearcher{err: errors.New("unavailable")}
	repository := NewElasticsearchRepository(searcher)

	_, err := repository.FindPatients(t.Context(), PatientSearchRequest{})
	if err == nil || err.Error() != "search patients in Elasticsearch: unavailable" {
		t.Fatalf("FindPatients() search error = %q", err)
	}

	searcher.err = nil
	searcher.documents = []json.RawMessage{json.RawMessage(`{"date_of_birth":false}`)}
	_, err = repository.FindPatients(t.Context(), PatientSearchRequest{})
	if err == nil || !strings.Contains(err.Error(), "decode patient search result 0 from Elasticsearch") {
		t.Fatalf("FindPatients() decode error = %q", err)
	}
}
