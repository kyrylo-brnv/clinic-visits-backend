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

func TestInitializeCreatesBootstrapIndexIdempotently(t *testing.T) {
	var mu sync.Mutex
	indexExists := false
	createCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/_cluster/health":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"status":"green","timed_out":false}`))
		case request.Method == http.MethodHead && request.URL.Path == "/"+BootstrapIndexName:
			mu.Lock()
			exists := indexExists
			mu.Unlock()
			if !exists {
				response.WriteHeader(http.StatusNotFound)
			}
		case request.Method == http.MethodPut && request.URL.Path == "/"+BootstrapIndexName:
			var body struct {
				Settings struct {
					Shards   int `json:"number_of_shards"`
					Replicas int `json:"number_of_replicas"`
				} `json:"settings"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode create index request: %v", err)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.Settings.Shards != 1 || body.Settings.Replicas != 0 {
				t.Errorf("create index settings = %+v", body.Settings)
			}

			mu.Lock()
			indexExists = true
			createCount++
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
	if createCount != 1 {
		t.Fatalf("index create count = %d, want 1", createCount)
	}
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
