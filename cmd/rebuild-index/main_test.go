package main

import (
	"strings"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

func TestRebuildIndexNameAcceptsSupportedIndex(t *testing.T) {
	t.Parallel()

	for _, indexName := range []string{
		elasticsearch.DoctorsIndexName,
		elasticsearch.PatientsIndexName,
		elasticsearch.ClinicsIndexName,
		elasticsearch.VisitsIndexName,
	} {
		indexName := indexName
		t.Run(indexName, func(t *testing.T) {
			t.Parallel()

			got, err := rebuildIndexName([]string{indexName})
			if err != nil {
				t.Fatalf("rebuildIndexName() error = %v", err)
			}
			if got != indexName {
				t.Fatalf("rebuildIndexName() = %q, want %q", got, indexName)
			}
		})
	}
}

func TestRebuildIndexNameRejectsMissingExtraAndUnsupportedInputs(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		arguments []string
		wantError string
	}{
		{name: "missing", wantError: "expected exactly one Elasticsearch index name"},
		{name: "extra", arguments: []string{elasticsearch.DoctorsIndexName, elasticsearch.PatientsIndexName}, wantError: "expected exactly one Elasticsearch index name"},
		{name: "unsupported", arguments: []string{"unknown-v1"}, wantError: `unsupported Elasticsearch index "unknown-v1"`},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := rebuildIndexName(testCase.arguments)
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("rebuildIndexName(%q) error = %v, want %q", testCase.arguments, err, testCase.wantError)
			}
		})
	}
}
