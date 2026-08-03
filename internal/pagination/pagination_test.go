package pagination_test

import (
	"net/url"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/pagination"
)

func TestParamsZeroValueUsesDefaults(t *testing.T) {
	var params pagination.Params

	if params.Page() != 1 {
		t.Fatalf("expected page 1, got %d", params.Page())
	}

	if params.Limit() != 20 {
		t.Fatalf("expected limit 20, got %d", params.Limit())
	}

	if params.Offset() != 0 {
		t.Fatalf("expected offset 0, got %d", params.Offset())
	}
}

func TestParsePagination(t *testing.T) {
	params, err := pagination.Parse(url.Values{
		"page":     {"3"},
		"per_page": {"50"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Page() != 3 {
		t.Fatalf("expected page 3, got %d", params.Page())
	}

	if params.PerPage() != 50 {
		t.Fatalf("expected per_page 50, got %d", params.PerPage())
	}

	if params.Offset() != 100 {
		t.Fatalf("expected offset 100, got %d", params.Offset())
	}
}

func TestParsePaginationRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		values url.Values
	}{
		{
			name:   "empty page",
			values: url.Values{"page": {""}},
		},
		{
			name:   "non-numeric page",
			values: url.Values{"page": {"abc"}},
		},
		{
			name:   "zero page",
			values: url.Values{"page": {"0"}},
		},
		{
			name:   "negative page",
			values: url.Values{"page": {"-1"}},
		},
		{
			name:   "zero per_page",
			values: url.Values{"per_page": {"0"}},
		},
		{
			name:   "per_page above maximum",
			values: url.Values{"per_page": {"201"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := pagination.Parse(test.values)
			if err == nil {
				t.Fatal("expected pagination error")
			}
		})
	}
}
