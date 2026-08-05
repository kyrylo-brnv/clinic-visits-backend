package elasticsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/config"
)

func TestNewClientRejectsInvalidURL(t *testing.T) {
	_, err := NewClient(&config.ElasticsearchConfig{URL: "localhost:9200"})
	if err == nil {
		t.Fatal("NewClient() error = nil, want invalid URL error")
	}
}

func TestInitializeCreatesVersionedIndicesIdempotently(t *testing.T) {
	var mu sync.Mutex
	indexExists := make(map[string]bool)
	createCounts := make(map[string]int)
	headCounts := make(map[string]int)

	indexNames := map[string]struct{}{
		DoctorsIndexName:  {},
		VisitsIndexName:   {},
		PatientsIndexName: {},
		ClinicsIndexName:  {},
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/_cluster/health":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"status":"green","timed_out":false}`))
		case request.Method == http.MethodHead:
			indexName := strings.TrimPrefix(request.URL.Path, "/")
			if _, ok := indexNames[indexName]; !ok {
				t.Errorf("unexpected index check path %q", request.URL.Path)
				response.WriteHeader(http.StatusNotFound)
				return
			}
			mu.Lock()
			exists := indexExists[indexName]
			headCounts[indexName]++
			mu.Unlock()
			if !exists {
				response.WriteHeader(http.StatusNotFound)
			}
		case request.Method == http.MethodPut:
			indexName := strings.TrimPrefix(request.URL.Path, "/")
			if _, ok := indexNames[indexName]; !ok {
				t.Errorf("unexpected index create path %q", request.URL.Path)
				response.WriteHeader(http.StatusNotFound)
				return
			}

			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode create index request: %v", err)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			assertIndexMapping(t, indexName, body)

			mu.Lock()
			indexExists[indexName] = true
			createCounts[indexName]++
			mu.Unlock()
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"acknowledged":true}`))
		default:
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(&config.ElasticsearchConfig{URL: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := client.Initialize(t.Context()); err != nil {
		t.Fatalf("first Initialize() error = %v", err)
	}
	if err := client.Initialize(t.Context()); err != nil {
		t.Fatalf("second Initialize() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for indexName := range indexNames {
		if createCounts[indexName] != 1 {
			t.Errorf("%s create count = %d, want 1", indexName, createCounts[indexName])
		}
		if headCounts[indexName] != 2 {
			t.Errorf("%s check count = %d, want 2", indexName, headCounts[indexName])
		}
	}
}

func TestInitializeSucceedsWhenIndexIsCreatedConcurrently(t *testing.T) {
	createAttempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/_cluster/health":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"status":"green","timed_out":false}`))
		case request.Method == http.MethodHead && request.URL.Path == "/"+DoctorsIndexName:
			response.WriteHeader(http.StatusNotFound)
		case request.Method == http.MethodHead:
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPut && request.URL.Path == "/"+DoctorsIndexName:
			createAttempts++
			response.WriteHeader(http.StatusBadRequest)
			_, _ = response.Write([]byte(`{"error":{"type":"resource_already_exists_exception","reason":"index already exists"},"status":400}`))
		default:
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(&config.ElasticsearchConfig{URL: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := client.Initialize(t.Context()); err != nil {
		t.Fatalf("Initialize() error = %v, want nil", err)
	}
	if createAttempts != 1 {
		t.Fatalf("doctors index create attempts = %d, want 1", createAttempts)
	}
}

func TestInitializeFailsForUnexpectedIndexCreationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/_cluster/health":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"status":"green","timed_out":false}`))
		case request.Method == http.MethodHead && request.URL.Path == "/"+DoctorsIndexName:
			response.WriteHeader(http.StatusNotFound)
		case request.Method == http.MethodPut && request.URL.Path == "/"+DoctorsIndexName:
			response.WriteHeader(http.StatusBadRequest)
			_, _ = response.Write([]byte(`{"error":{"type":"illegal_argument_exception","reason":"invalid mapping"},"status":400}`))
		default:
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(&config.ElasticsearchConfig{URL: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.Initialize(t.Context())
	if err == nil {
		t.Fatal("Initialize() error = nil, want index creation error")
	}
	if !strings.Contains(err.Error(), "unexpected response status 400 Bad Request") {
		t.Fatalf("Initialize() error = %q", err)
	}
}

func assertIndexMapping(t *testing.T, indexName string, body map[string]any) {
	t.Helper()

	settings := objectValue(t, body, "settings")
	if settings["number_of_shards"] != float64(1) || settings["number_of_replicas"] != float64(0) {
		t.Errorf("%s settings = %#v, want one shard and zero replicas", indexName, settings)
	}

	properties := objectValue(t, objectValue(t, body, "mappings"), "properties")
	switch indexName {
	case DoctorsIndexName:
		assertProperties(t, properties, map[string]string{
			"id": "keyword", "specialty_id": "keyword", "clinic_id": "keyword",
			"full_name": "text", "created_at": "date", "updated_at": "date",
		})
		assertTextKeyword(t, properties, "full_name")
		assertNestedVisits(t, properties)
	case VisitsIndexName:
		assertProperties(t, properties, map[string]string{
			"id": "keyword", "doctor_id": "keyword", "patient_id": "keyword", "clinic_id": "keyword",
			"status": "keyword", "visit_start_time": "date", "visit_end_time": "date",
			"created_at": "date", "updated_at": "date",
		})
		assertProperties(t, objectValue(t, objectValue(t, properties, "doctor"), "properties"), map[string]string{
			"id": "keyword", "specialty_id": "keyword", "clinic_id": "keyword", "full_name": "text",
		})
		assertTextKeyword(t, objectValue(t, objectValue(t, properties, "doctor"), "properties"), "full_name")
		assertProperties(t, objectValue(t, objectValue(t, properties, "patient"), "properties"), map[string]string{
			"id": "keyword", "first_name": "text", "last_name": "text", "date_of_birth": "date", "gender": "keyword", "is_deleted": "boolean",
		})
		assertTextKeyword(t, objectValue(t, objectValue(t, properties, "patient"), "properties"), "first_name")
		assertTextKeyword(t, objectValue(t, objectValue(t, properties, "patient"), "properties"), "last_name")
		assertProperties(t, objectValue(t, objectValue(t, properties, "clinic"), "properties"), map[string]string{
			"id": "keyword", "name": "text", "address": "text", "time_zone": "keyword",
		})
		assertTextKeyword(t, objectValue(t, objectValue(t, properties, "clinic"), "properties"), "name")
		assertTextKeyword(t, objectValue(t, objectValue(t, properties, "clinic"), "properties"), "address")
	case PatientsIndexName:
		assertProperties(t, properties, map[string]string{
			"id": "keyword", "first_name": "text", "last_name": "text", "date_of_birth": "date",
			"gender": "keyword", "is_deleted": "boolean", "created_at": "date", "updated_at": "date",
		})
		assertTextKeyword(t, properties, "first_name")
		assertTextKeyword(t, properties, "last_name")
		assertNestedVisits(t, properties)
	case ClinicsIndexName:
		assertProperties(t, properties, map[string]string{
			"id": "keyword", "name": "text", "address": "text", "time_zone": "keyword",
			"created_at": "date", "updated_at": "date",
		})
		assertTextKeyword(t, properties, "name")
		assertTextKeyword(t, properties, "address")
		assertNestedVisits(t, properties)
	default:
		t.Errorf("unexpected index %q", indexName)
	}
}

func assertNestedVisits(t *testing.T, properties map[string]any) {
	t.Helper()

	visits := objectValue(t, properties, "visits")
	if visits["type"] != "nested" {
		t.Errorf("visits type = %#v, want nested", visits["type"])
	}
	assertProperties(t, objectValue(t, visits, "properties"), map[string]string{
		"id": "keyword", "doctor_id": "keyword", "patient_id": "keyword", "clinic_id": "keyword",
		"status": "keyword", "visit_start_time": "date", "visit_end_time": "date",
		"created_at": "date", "updated_at": "date",
	})
}

func assertProperties(t *testing.T, properties map[string]any, expected map[string]string) {
	t.Helper()

	for field, fieldType := range expected {
		property := objectValue(t, properties, field)
		if property["type"] != fieldType {
			t.Errorf("%s type = %#v, want %q", field, property["type"], fieldType)
		}
	}
}

func assertTextKeyword(t *testing.T, properties map[string]any, field string) {
	t.Helper()

	keyword := objectValue(t, objectValue(t, objectValue(t, properties, field), "fields"), "keyword")
	if keyword["type"] != "keyword" {
		t.Errorf("%s.keyword type = %#v, want keyword", field, keyword["type"])
	}
}

func objectValue(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := object[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", key, object[key])
	}
	return value
}

func TestInitializeFailsWhenElasticsearchIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	client, err := NewClient(&config.ElasticsearchConfig{URL: url})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.Initialize(context.Background())
	if err == nil {
		t.Fatal("Initialize() error = nil, want unavailable error")
	}
	if !strings.Contains(err.Error(), "check elasticsearch health") {
		t.Fatalf("Initialize() error = %q", err)
	}
}

func TestInitializeRejectsUnhealthyCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/_cluster/health" {
			t.Errorf("request path = %q, want cluster health path", request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"status":"red","timed_out":false}`))
	}))
	defer server.Close()

	client, err := NewClient(&config.ElasticsearchConfig{URL: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.Initialize(t.Context())
	if err == nil {
		t.Fatal("Initialize() error = nil, want unhealthy cluster error")
	}
	if !strings.Contains(err.Error(), `cluster health status is "red"`) {
		t.Fatalf("Initialize() error = %q", err)
	}
}
