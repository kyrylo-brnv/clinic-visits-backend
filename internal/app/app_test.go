package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/config"
	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

func TestNewComposesV2VisitListWithElasticsearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/visits-v1/_search" {
			t.Fatalf("Elasticsearch path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("Elasticsearch method = %s", r.Method)
		}

		var query map[string]any
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			t.Fatalf("decode search query: %v", err)
		}
		if query["from"] != float64(0) || query["size"] != float64(20) {
			t.Fatalf("search pagination = %#v", query)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"hits":[{"_source":{"id":"visit-1","doctor_id":"doctor-1","patient_id":"patient-1","clinic_id":"clinic-1","status":"SCHEDULED","visit_start_time":"2026-08-05T09:00:00Z","visit_end_time":"2026-08-05T10:00:00Z","created_at":"2026-08-04T12:00:00Z","updated_at":"2026-08-04T12:00:00Z"}}]}}`))
	}))
	defer server.Close()

	client, err := elasticsearch.NewClient(&config.ElasticsearchConfig{URL: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	handler := New(nil, client)
	request := httptest.NewRequest(http.MethodPost, "/v2/visits/list", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("POST /v2/visits/list status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != `{"data":[{"id":"visit-1","doctor_id":"doctor-1","patient_id":"patient-1","clinic_id":"clinic-1","status":"SCHEDULED","visit_start_time":"2026-08-05T09:00:00.000000+00:00","visit_end_time":"2026-08-05T10:00:00.000000+00:00","created_at":"2026-08-04T12:00:00.000000+00:00","updated_at":"2026-08-04T12:00:00.000000+00:00"}]}` {
		t.Fatalf("POST /v2/visits/list body = %s", response.Body.String())
	}
}
