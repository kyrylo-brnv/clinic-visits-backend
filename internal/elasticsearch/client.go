package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/smithautotest/clinic-visits/internal/config"
)

const (
	DoctorsIndexName  = "doctors-v1"
	VisitsIndexName   = "visits-v1"
	PatientsIndexName = "patients-v1"
	ClinicsIndexName  = "clinics-v1"
)

type indexDefinition struct {
	name string
	body string
}

var indexDefinitions = []indexDefinition{
	{
		name: DoctorsIndexName,
		body: `{
  "settings": {"number_of_shards": 1, "number_of_replicas": 0},
  "mappings": {
    "properties": {
      "id": {"type": "keyword"},
      "specialty_id": {"type": "keyword"},
      "clinic_id": {"type": "keyword"},
      "full_name": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
      "created_at": {"type": "date"},
      "updated_at": {"type": "date"},
      "visits": {
        "type": "nested",
        "properties": {
          "id": {"type": "keyword"},
          "doctor_id": {"type": "keyword"},
          "patient_id": {"type": "keyword"},
          "clinic_id": {"type": "keyword"},
          "status": {"type": "keyword"},
          "visit_start_time": {"type": "date"},
          "visit_end_time": {"type": "date"},
          "created_at": {"type": "date"},
          "updated_at": {"type": "date"}
        }
      }
    }
  }
}`,
	},
	{
		name: VisitsIndexName,
		body: `{
  "settings": {"number_of_shards": 1, "number_of_replicas": 0},
  "mappings": {
    "properties": {
      "id": {"type": "keyword"},
      "doctor_id": {"type": "keyword"},
      "patient_id": {"type": "keyword"},
      "clinic_id": {"type": "keyword"},
      "status": {"type": "keyword"},
      "visit_start_time": {"type": "date"},
      "visit_end_time": {"type": "date"},
      "created_at": {"type": "date"},
      "updated_at": {"type": "date"},
      "doctor": {
        "properties": {
          "id": {"type": "keyword"},
          "specialty_id": {"type": "keyword"},
          "clinic_id": {"type": "keyword"},
          "full_name": {"type": "text", "fields": {"keyword": {"type": "keyword"}}}
        }
      },
      "patient": {
        "properties": {
          "id": {"type": "keyword"},
          "first_name": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
          "last_name": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
          "date_of_birth": {"type": "date"},
          "gender": {"type": "keyword"},
          "is_deleted": {"type": "boolean"}
        }
      },
      "clinic": {
        "properties": {
          "id": {"type": "keyword"},
          "name": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
          "address": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
          "time_zone": {"type": "keyword"}
        }
      }
    }
  }
}`,
	},
	{
		name: PatientsIndexName,
		body: `{
  "settings": {"number_of_shards": 1, "number_of_replicas": 0},
  "mappings": {
    "properties": {
      "id": {"type": "keyword"},
      "first_name": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
      "last_name": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
      "date_of_birth": {"type": "date"},
      "gender": {"type": "keyword"},
      "is_deleted": {"type": "boolean"},
      "created_at": {"type": "date"},
      "updated_at": {"type": "date"},
      "visits": {
        "type": "nested",
        "properties": {
          "id": {"type": "keyword"},
          "doctor_id": {"type": "keyword"},
          "patient_id": {"type": "keyword"},
          "clinic_id": {"type": "keyword"},
          "status": {"type": "keyword"},
          "visit_start_time": {"type": "date"},
          "visit_end_time": {"type": "date"},
          "created_at": {"type": "date"},
          "updated_at": {"type": "date"}
        }
      }
    }
  }
}`,
	},
	{
		name: ClinicsIndexName,
		body: `{
  "settings": {"number_of_shards": 1, "number_of_replicas": 0},
  "mappings": {
    "properties": {
      "id": {"type": "keyword"},
      "name": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
      "address": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
      "time_zone": {"type": "keyword"},
      "created_at": {"type": "date"},
      "updated_at": {"type": "date"},
      "visits": {
        "type": "nested",
        "properties": {
          "id": {"type": "keyword"},
          "doctor_id": {"type": "keyword"},
          "patient_id": {"type": "keyword"},
          "clinic_id": {"type": "keyword"},
          "status": {"type": "keyword"},
          "visit_start_time": {"type": "date"},
          "visit_end_time": {"type": "date"},
          "created_at": {"type": "date"},
          "updated_at": {"type": "date"}
        }
      }
    }
  }
}`,
	},
}

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(cfg *config.ElasticsearchConfig) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("elasticsearch config must not be nil")
	}

	baseURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse elasticsearch URL: %w", err)
	}
	if (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return nil, fmt.Errorf("elasticsearch URL must be an absolute HTTP or HTTPS URL")
	}
	if (baseURL.Path != "" && baseURL.Path != "/") || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, fmt.Errorf("elasticsearch URL must not contain a path, query, or fragment")
	}

	baseURL.Path = ""
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

func (c *Client) Initialize(ctx context.Context) error {
	if err := c.checkHealth(ctx); err != nil {
		return fmt.Errorf("check elasticsearch health: %w", err)
	}
	for _, definition := range indexDefinitions {
		if err := c.ensureIndex(ctx, definition); err != nil {
			return fmt.Errorf("ensure elasticsearch index %q: %w", definition.name, err)
		}
	}

	return nil
}

func (c *Client) checkHealth(ctx context.Context) error {
	request, err := c.newRequest(ctx, http.MethodGet, "/_cluster/health", nil)
	if err != nil {
		return err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request cluster health: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return responseStatusError(response)
	}

	var health struct {
		Status   string `json:"status"`
		TimedOut bool   `json:"timed_out"`
	}
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		return fmt.Errorf("decode cluster health: %w", err)
	}
	if health.TimedOut {
		return fmt.Errorf("cluster health request timed out")
	}
	if health.Status != "green" && health.Status != "yellow" {
		return fmt.Errorf("cluster health status is %q", health.Status)
	}

	return nil
}

func (c *Client) ensureIndex(ctx context.Context, definition indexDefinition) error {
	request, err := c.newRequest(ctx, http.MethodHead, "/"+definition.name, nil)
	if err != nil {
		return err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("check index: %w", err)
	}

	if response.StatusCode == http.StatusOK {
		response.Body.Close()
		return nil
	}
	if response.StatusCode != http.StatusNotFound {
		defer response.Body.Close()
		return responseStatusError(response)
	}
	response.Body.Close()

	body := strings.NewReader(definition.body)
	request, err = c.newRequest(ctx, http.MethodPut, "/"+definition.name, body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err = c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response.StatusCode == http.StatusBadRequest {
			body, err := io.ReadAll(io.LimitReader(response.Body, 4<<10))
			if err != nil {
				return fmt.Errorf("unexpected response status %s (read response: %w)", response.Status, err)
			}
			if isIndexAlreadyExistsError(body) {
				return nil
			}
			return responseStatusErrorWithBody(response.Status, body)
		}
		return responseStatusError(response)
	}

	var result struct {
		Acknowledged bool `json:"acknowledged"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode create index response: %w", err)
	}
	if !result.Acknowledged {
		return fmt.Errorf("index creation was not acknowledged")
	}

	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	endpoint := *c.baseURL
	endpoint.Path = path

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	return request, nil
}

func responseStatusError(response *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<10))
	if err != nil {
		return fmt.Errorf("unexpected response status %s (read response: %w)", response.Status, err)
	}

	return responseStatusErrorWithBody(response.Status, body)
}

func responseStatusErrorWithBody(status string, body []byte) error {
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("unexpected response status %s", status)
	}

	return fmt.Errorf("unexpected response status %s: %s", status, detail)
}

func isIndexAlreadyExistsError(body []byte) bool {
	var response struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return false
	}

	return response.Error.Type == "resource_already_exists_exception"
}
