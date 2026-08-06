package elasticsearch

import (
	"bytes"
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

// UpsertDocument creates or replaces one document in an Elasticsearch index.
func (c *Client) UpsertDocument(ctx context.Context, indexName, documentID string, document any) error {
	if strings.TrimSpace(indexName) == "" {
		return fmt.Errorf("elasticsearch index name must not be blank")
	}
	if strings.TrimSpace(documentID) == "" {
		return fmt.Errorf("elasticsearch document ID must not be blank")
	}

	body, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("marshal document for Elasticsearch index %q with ID %q: %w", indexName, documentID, err)
	}

	request, err := c.newDocumentRequest(ctx, http.MethodPut, indexName, documentID, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create document upsert request for Elasticsearch index %q with ID %q: %w", indexName, documentID, err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("upsert document in Elasticsearch index %q with ID %q: %w", indexName, documentID, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("upsert document in Elasticsearch index %q with ID %q: unexpected response status %s", indexName, documentID, response.Status)
	}

	return nil
}

// GetDocument loads one document source from an Elasticsearch index. A missing
// document is reported as found=false without an error.
func (c *Client) GetDocument(ctx context.Context, indexName, documentID string, document any) (bool, error) {
	if strings.TrimSpace(indexName) == "" {
		return false, fmt.Errorf("elasticsearch index name must not be blank")
	}
	if strings.TrimSpace(documentID) == "" {
		return false, fmt.Errorf("elasticsearch document ID must not be blank")
	}
	if document == nil {
		return false, fmt.Errorf("elasticsearch document destination must not be nil")
	}

	request, err := c.newDocumentRequest(ctx, http.MethodGet, indexName, documentID, nil)
	if err != nil {
		return false, fmt.Errorf("create document get request for Elasticsearch index %q with ID %q: %w", indexName, documentID, err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("get document from Elasticsearch index %q with ID %q: %w", indexName, documentID, err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return false, fmt.Errorf("get document from Elasticsearch index %q with ID %q: unexpected response status %s", indexName, documentID, response.Status)
	}

	var result struct {
		Source json.RawMessage `json:"_source"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode document from Elasticsearch index %q with ID %q: %w", indexName, documentID, err)
	}
	if len(result.Source) == 0 {
		return false, fmt.Errorf("decode document from Elasticsearch index %q with ID %q: response is missing _source", indexName, documentID)
	}
	if err := json.Unmarshal(result.Source, document); err != nil {
		return false, fmt.Errorf("decode document source from Elasticsearch index %q with ID %q: %w", indexName, documentID, err)
	}

	return true, nil
}

// Search returns the _source value from every hit matching query in indexName.
func (c *Client) Search(ctx context.Context, indexName string, query any) ([]json.RawMessage, error) {
	if strings.TrimSpace(indexName) == "" {
		return nil, fmt.Errorf("elasticsearch index name must not be blank")
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal search query for Elasticsearch index %q: %w", indexName, err)
	}

	request, err := c.newSearchRequest(ctx, indexName, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create search request for Elasticsearch index %q: %w", indexName, err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("search Elasticsearch index %q: %w", indexName, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("search Elasticsearch index %q: unexpected response status %s", indexName, response.Status)
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search response from Elasticsearch index %q: %w", indexName, err)
	}

	documents := make([]json.RawMessage, 0, len(result.Hits.Hits))
	for hitIndex, hit := range result.Hits.Hits {
		if len(hit.Source) == 0 {
			return nil, fmt.Errorf("decode search hit %d from Elasticsearch index %q: response is missing _source", hitIndex, indexName)
		}
		documents = append(documents, hit.Source)
	}

	return documents, nil
}

// DeleteDocument removes one document from an Elasticsearch index. Deleting an
// already missing document succeeds so outbox retries remain idempotent.
func (c *Client) DeleteDocument(ctx context.Context, indexName, documentID string) error {
	if strings.TrimSpace(indexName) == "" {
		return fmt.Errorf("elasticsearch index name must not be blank")
	}
	if strings.TrimSpace(documentID) == "" {
		return fmt.Errorf("elasticsearch document ID must not be blank")
	}

	request, err := c.newDocumentRequest(ctx, http.MethodDelete, indexName, documentID, nil)
	if err != nil {
		return fmt.Errorf("create document delete request for Elasticsearch index %q with ID %q: %w", indexName, documentID, err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("delete document from Elasticsearch index %q with ID %q: %w", indexName, documentID, err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("delete document from Elasticsearch index %q with ID %q: unexpected response status %s", indexName, documentID, response.Status)
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

func (c *Client) newDocumentRequest(ctx context.Context, method, indexName, documentID string, body io.Reader) (*http.Request, error) {
	endpoint := *c.baseURL
	endpoint.Path = "/" + indexName + "/_doc/" + documentID
	endpoint.RawPath = "/" + url.PathEscape(indexName) + "/_doc/" + url.PathEscape(documentID)

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	return request, nil
}

func (c *Client) newSearchRequest(ctx context.Context, indexName string, body io.Reader) (*http.Request, error) {
	endpoint := *c.baseURL
	endpoint.Path = "/" + indexName + "/_search"
	endpoint.RawPath = "/" + url.PathEscape(indexName) + "/_search"

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), body)
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
