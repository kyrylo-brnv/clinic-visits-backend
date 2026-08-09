package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

const validSyncIndexID = "018f6e4d-2c5f-7f41-bba4-01af8baf241b"

func TestSyncIndexArgumentsAcceptsSupportedIndexAndCanonicalIDs(t *testing.T) {
	t.Parallel()

	ids := []string{
		validSyncIndexID,
		"f2f8b42b-7d3a-4e6e-8e70-3a44d2d2a391",
	}
	for _, indexName := range []string{
		elasticsearch.DoctorsIndexName,
		elasticsearch.PatientsIndexName,
		elasticsearch.ClinicsIndexName,
		elasticsearch.VisitsIndexName,
	} {
		indexName := indexName
		t.Run(indexName, func(t *testing.T) {
			t.Parallel()

			gotIndexName, gotIDs, err := syncIndexArguments(append([]string{indexName}, ids...))
			if err != nil {
				t.Fatalf("syncIndexArguments() error = %v", err)
			}
			if gotIndexName != indexName {
				t.Fatalf("syncIndexArguments() index name = %q, want %q", gotIndexName, indexName)
			}
			if !reflect.DeepEqual(gotIDs, ids) {
				t.Fatalf("syncIndexArguments() IDs = %q, want %q", gotIDs, ids)
			}
		})
	}
}

func TestSyncIndexArgumentsRejectsInvalidArgumentShapes(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		arguments []string
		wantError string
	}{
		{name: "missing index", wantError: "expected an Elasticsearch index name"},
		{name: "missing IDs", arguments: []string{elasticsearch.DoctorsIndexName}, wantError: "expected at least one PostgreSQL UUID ID"},
		{name: "unsupported index", arguments: []string{"unknown-v1", validSyncIndexID}, wantError: `unsupported Elasticsearch index "unknown-v1"`},
		{name: "invalid ID", arguments: []string{elasticsearch.DoctorsIndexName, "not-a-uuid"}, wantError: `invalid PostgreSQL UUID ID "not-a-uuid"`},
		{name: "uppercase ID", arguments: []string{elasticsearch.DoctorsIndexName, strings.ToUpper(validSyncIndexID)}, wantError: "expected canonical UUID format"},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := syncIndexArguments(testCase.arguments)
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("syncIndexArguments(%q) error = %v, want %q", testCase.arguments, err, testCase.wantError)
			}
		})
	}
}

func TestSyncIndexArgumentsEnforcesMaximumIDs(t *testing.T) {
	t.Parallel()

	maximumIDs := make([]string, elasticsearch.MaxSyncIndexIDs)
	for i := range maximumIDs {
		maximumIDs[i] = validSyncIndexID
	}

	if _, gotIDs, err := syncIndexArguments(append([]string{elasticsearch.DoctorsIndexName}, maximumIDs...)); err != nil {
		t.Fatalf("syncIndexArguments() at maximum IDs error = %v", err)
	} else if len(gotIDs) != elasticsearch.MaxSyncIndexIDs {
		t.Fatalf("syncIndexArguments() at maximum IDs returned %d IDs, want %d", len(gotIDs), elasticsearch.MaxSyncIndexIDs)
	}

	tooManyIDs := append(maximumIDs, validSyncIndexID)
	if _, _, err := syncIndexArguments(append([]string{elasticsearch.DoctorsIndexName}, tooManyIDs...)); err == nil || !strings.Contains(err.Error(), "expected at most 100 PostgreSQL UUID IDs") {
		t.Fatalf("syncIndexArguments() beyond maximum IDs error = %v, want maximum-ID error", err)
	}
}
