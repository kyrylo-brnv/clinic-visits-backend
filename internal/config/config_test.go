package config

import (
	"strings"
	"testing"
)

func TestLoadElasticsearchConfig(t *testing.T) {
	t.Setenv("ELASTICSEARCH_URL", "http://localhost:9200")

	cfg, err := LoadElasticsearchConfig()
	if err != nil {
		t.Fatalf("LoadElasticsearchConfig() error = %v", err)
	}
	if cfg.URL != "http://localhost:9200" {
		t.Fatalf("LoadElasticsearchConfig() URL = %q, want %q", cfg.URL, "http://localhost:9200")
	}
}

func TestLoadElasticsearchConfigRequiresURL(t *testing.T) {
	t.Setenv("ELASTICSEARCH_URL", "")

	_, err := LoadElasticsearchConfig()
	if err == nil {
		t.Fatal("LoadElasticsearchConfig() error = nil, want missing configuration error")
	}
	if !strings.Contains(err.Error(), "ELASTICSEARCH_URL is not configured") {
		t.Fatalf("LoadElasticsearchConfig() error = %q", err)
	}
}
