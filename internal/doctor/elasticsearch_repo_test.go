package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

type fakeDoctorDocumentSearcher struct {
	indexName string
	query     any
	documents []json.RawMessage
	err       error
}

func (s *fakeDoctorDocumentSearcher) Search(_ context.Context, indexName string, query any) ([]json.RawMessage, error) {
	s.indexName = indexName
	s.query = query
	return s.documents, s.err
}

func TestElasticsearchRepositoryFindDoctorsBuildsV1CompatibleQuery(t *testing.T) {
	searcher := &fakeDoctorDocumentSearcher{
		documents: []json.RawMessage{
			json.RawMessage(`{"id":"doctor-1","specialty_id":"specialty-1","clinic_id":"clinic-1","full_name":"Jane Doe"}`),
		},
	}
	repository := NewElasticsearchRepository(searcher)

	doctors, err := repository.FindDoctors(t.Context(), DoctorSearchRequest{
		Filter: &DoctorFilter{
			DoctorID: "doctor-1",
			VisitID:  "visit-1",
			ClinicID: "clinic-1",
		},
	})
	if err != nil {
		t.Fatalf("FindDoctors() error = %v", err)
	}

	if searcher.indexName != elasticsearch.DoctorsIndexName {
		t.Fatalf("Search() index = %q, want %q", searcher.indexName, elasticsearch.DoctorsIndexName)
	}
	if !reflect.DeepEqual(searcher.query, map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{
					map[string]any{"term": map[string]string{"id": "doctor-1"}},
					map[string]any{
						"nested": map[string]any{
							"path": "visits",
							"query": map[string]any{
								"term": map[string]string{"visits.id": "visit-1"},
							},
						},
					},
					map[string]any{"term": map[string]string{"clinic_id": "clinic-1"}},
				},
			},
		},
		"sort": []map[string]map[string]string{
			{"full_name.keyword": {"order": "asc"}},
			{"id": {"order": "asc"}},
		},
	}) {
		t.Fatalf("Search() query = %#v", searcher.query)
	}

	wantDoctors := []Doctor{{
		ID:          "doctor-1",
		SpecialtyID: "specialty-1",
		ClinicID:    "clinic-1",
		FullName:    "Jane Doe",
	}}
	if !reflect.DeepEqual(doctors, wantDoctors) {
		t.Fatalf("FindDoctors() = %#v, want %#v", doctors, wantDoctors)
	}
}

func TestElasticsearchRepositoryFindDoctorsAllowsNoFilters(t *testing.T) {
	searcher := &fakeDoctorDocumentSearcher{}
	repository := NewElasticsearchRepository(searcher)

	_, err := repository.FindDoctors(t.Context(), DoctorSearchRequest{})
	if err != nil {
		t.Fatalf("FindDoctors() error = %v", err)
	}

	wantQuery := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{},
			},
		},
		"sort": []map[string]map[string]string{
			{"full_name.keyword": {"order": "asc"}},
			{"id": {"order": "asc"}},
		},
	}
	if !reflect.DeepEqual(searcher.query, wantQuery) {
		t.Fatalf("Search() query = %#v, want %#v", searcher.query, wantQuery)
	}
}

func TestElasticsearchRepositoryFindDoctorsReportsSearchAndDecodeErrors(t *testing.T) {
	searcher := &fakeDoctorDocumentSearcher{err: errors.New("unavailable")}
	repository := NewElasticsearchRepository(searcher)

	_, err := repository.FindDoctors(t.Context(), DoctorSearchRequest{})
	if err == nil || err.Error() != "search doctors in Elasticsearch: unavailable" {
		t.Fatalf("FindDoctors() search error = %q", err)
	}

	searcher.err = nil
	searcher.documents = []json.RawMessage{json.RawMessage(`{"created_at":false}`)}
	_, err = repository.FindDoctors(t.Context(), DoctorSearchRequest{})
	if err == nil || !strings.Contains(err.Error(), "decode doctor search result 0 from Elasticsearch") {
		t.Fatalf("FindDoctors() decode error = %q", err)
	}
}
