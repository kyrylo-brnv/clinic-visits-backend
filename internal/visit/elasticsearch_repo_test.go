package visit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
	"github.com/smithautotest/clinic-visits/internal/pagination"
)

type fakeVisitDocumentSearcher struct {
	indexName string
	query     any
	queries   []any
	documents []json.RawMessage
	responses [][]json.RawMessage
	err       error
}

func (s *fakeVisitDocumentSearcher) Search(_ context.Context, indexName string, query any) ([]json.RawMessage, error) {
	s.indexName = indexName
	s.query = query
	s.queries = append(s.queries, query)
	if len(s.responses) > 0 {
		response := s.responses[0]
		s.responses = s.responses[1:]
		return response, s.err
	}
	return s.documents, s.err
}

func TestElasticsearchRepositoryListVisitsBuildsV1CompatibleQueryAndMapsDocuments(t *testing.T) {
	paginationParams, err := pagination.Parse(url.Values{"page": {"3"}, "per_page": {"50"}})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	searcher := &fakeVisitDocumentSearcher{documents: []json.RawMessage{json.RawMessage(`{
		"id":"visit-1",
		"doctor_id":"doctor-1",
		"patient_id":"patient-1",
		"clinic_id":"clinic-1",
		"status":"IN_PROGRESS",
		"visit_start_time":"2026-08-05T09:00:00.000000+03:00",
		"visit_end_time":"2026-08-05T10:00:00.000000+03:00",
		"created_at":"2026-08-04T12:00:00.123456+03:00",
		"updated_at":"2026-08-04T13:00:00.654321+03:00"
	}`)}}
	repository := NewElasticsearchRepository(searcher)

	visits, err := repository.ListVisits(t.Context(), ListVisitsRequest{Pagination: paginationParams})
	if err != nil {
		t.Fatalf("ListVisits() error = %v", err)
	}

	if searcher.indexName != elasticsearch.VisitsIndexName {
		t.Fatalf("Search() index = %q, want %q", searcher.indexName, elasticsearch.VisitsIndexName)
	}
	if !reflect.DeepEqual(searcher.query, map[string]any{
		"from": int64(100),
		"size": int32(50),
		"sort": []map[string]map[string]string{
			{"visit_start_time": {"order": "asc"}},
			{"id": {"order": "asc"}},
		},
	}) {
		t.Fatalf("Search() query = %#v", searcher.query)
	}

	wantTime := func(value string) time.Time {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			t.Fatalf("parse time %q: %v", value, err)
		}
		return parsed
	}
	wantVisits := []Visit{{
		ID:             "visit-1",
		DoctorID:       "doctor-1",
		PatientID:      "patient-1",
		ClinicID:       "clinic-1",
		Status:         StatusInProgress,
		VisitStartTime: wantTime("2026-08-05T09:00:00.000000+03:00"),
		VisitEndTime:   wantTime("2026-08-05T10:00:00.000000+03:00"),
		CreatedAt:      wantTime("2026-08-04T12:00:00.123456+03:00"),
		UpdatedAt:      wantTime("2026-08-04T13:00:00.654321+03:00"),
	}}
	if !reflect.DeepEqual(visits, wantVisits) {
		t.Fatalf("ListVisits() = %#v, want %#v", visits, wantVisits)
	}
}

func TestElasticsearchRepositoryListVisitsPaginatesBeyondResultWindow(t *testing.T) {
	paginationParams, err := pagination.Parse(url.Values{"page": {"51"}, "per_page": {"200"}})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	skippedDocuments := make([]json.RawMessage, elasticsearchMaxResultWindow)
	var lastSkipped Visit
	for index := range skippedDocuments {
		visit := Visit{
			ID:             fmt.Sprintf("visit-%05d", index),
			VisitStartTime: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(index) * time.Minute),
		}
		if index == len(skippedDocuments)-1 {
			lastSkipped = visit
		}

		document, err := json.Marshal(elasticsearch.VisitDocument{
			ID:             visit.ID,
			VisitStartTime: visit.VisitStartTime,
		})
		if err != nil {
			t.Fatalf("marshal skipped visit %d: %v", index, err)
		}
		skippedDocuments[index] = document
	}

	searcher := &fakeVisitDocumentSearcher{responses: [][]json.RawMessage{
		skippedDocuments,
		{json.RawMessage(`{"id":"visit-10000","doctor_id":"doctor-1","patient_id":"patient-1","clinic_id":"clinic-1","status":"SCHEDULED","visit_start_time":"2026-08-08T00:40:00Z","visit_end_time":"2026-08-08T01:40:00Z","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)},
	}}
	repository := NewElasticsearchRepository(searcher)

	visits, err := repository.ListVisits(t.Context(), ListVisitsRequest{Pagination: paginationParams})
	if err != nil {
		t.Fatalf("ListVisits() error = %v", err)
	}
	if len(visits) != 1 || visits[0].ID != "visit-10000" {
		t.Fatalf("ListVisits() = %#v", visits)
	}

	wantQueries := []any{
		map[string]any{
			"size": int32(elasticsearchMaxResultWindow),
			"sort": []map[string]map[string]string{
				{"visit_start_time": {"order": "asc"}},
				{"id": {"order": "asc"}},
			},
		},
		map[string]any{
			"size": int32(200),
			"sort": []map[string]map[string]string{
				{"visit_start_time": {"order": "asc"}},
				{"id": {"order": "asc"}},
			},
			"search_after": visitSearchAfterValues(lastSkipped),
		},
	}
	if !reflect.DeepEqual(searcher.queries, wantQueries) {
		t.Fatalf("Search() queries = %#v, want %#v", searcher.queries, wantQueries)
	}
}

func TestElasticsearchRepositoryListVisitsReportsSearchAndDecodeErrors(t *testing.T) {
	searcher := &fakeVisitDocumentSearcher{err: errors.New("unavailable")}
	repository := NewElasticsearchRepository(searcher)

	_, err := repository.ListVisits(t.Context(), ListVisitsRequest{})
	if err == nil || err.Error() != "search visits in Elasticsearch: unavailable" {
		t.Fatalf("ListVisits() search error = %q", err)
	}

	searcher.err = nil
	searcher.documents = []json.RawMessage{json.RawMessage(`{"visit_start_time":false}`)}
	_, err = repository.ListVisits(t.Context(), ListVisitsRequest{})
	if err == nil || !strings.Contains(err.Error(), "decode visit list result 0 from Elasticsearch") {
		t.Fatalf("ListVisits() decode error = %q", err)
	}
}
