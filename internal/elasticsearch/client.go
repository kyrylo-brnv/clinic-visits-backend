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

const BootstrapIndexName = "clinic-visits-bootstrap-v1"

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
	if err := c.ensureIndex(ctx, BootstrapIndexName); err != nil {
		return fmt.Errorf("ensure elasticsearch index %q: %w", BootstrapIndexName, err)
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

func (c *Client) ensureIndex(ctx context.Context, name string) error {
	request, err := c.newRequest(ctx, http.MethodHead, "/"+name, nil)
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

	body := strings.NewReader(`{"settings":{"number_of_shards":1,"number_of_replicas":0}}`)
	request, err = c.newRequest(ctx, http.MethodPut, "/"+name, body)
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

	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("unexpected response status %s", response.Status)
	}

	return fmt.Errorf("unexpected response status %s: %s", response.Status, detail)
}
