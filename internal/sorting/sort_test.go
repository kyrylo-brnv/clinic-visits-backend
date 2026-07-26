package sorting_test

import (
	"testing"

	"github.com/smithautotest/clinic-visits/internal/sorting"
)

func TestConfigAcceptsAllowedDescendingSort(t *testing.T) {
	config := sorting.NewAllowedFields("first_name")

	sort := &sorting.Sort{
		Field:     "first_name",
		Direction: sorting.Direction("desc"),
	}

	if !config.IsValid(sort) {
		t.Fatal("expected descending sort to be valid")
	}
}
