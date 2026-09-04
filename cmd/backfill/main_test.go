package main

import (
	"errors"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

func TestParseBackfillOptions(t *testing.T) {
	t.Parallel()

	options, err := parseBackfillOptions([]string{"-batch-size=25", "-concurrency=6"})
	if err != nil {
		t.Fatalf("parseBackfillOptions() error = %v", err)
	}
	if options.BatchSize != 25 || options.Concurrency != 6 {
		t.Fatalf("parseBackfillOptions() = %+v, want batch size 25 and concurrency 6", options)
	}
}

func TestParseBackfillOptionsUsesBoundedDefaults(t *testing.T) {
	t.Parallel()

	options, err := parseBackfillOptions(nil)
	if err != nil {
		t.Fatalf("parseBackfillOptions() error = %v", err)
	}
	if options != elasticsearch.DefaultBackfillOptions() {
		t.Fatalf("parseBackfillOptions() = %+v, want %+v", options, elasticsearch.DefaultBackfillOptions())
	}
}

func TestParseBackfillOptionsRejectsInvalidLimitsAndArguments(t *testing.T) {
	t.Parallel()

	if _, err := parseBackfillOptions([]string{"-concurrency=0"}); !errors.Is(err, elasticsearch.ErrInvalidBackfillConcurrency) {
		t.Fatalf("zero concurrency error = %v", err)
	}
	if _, err := parseBackfillOptions([]string{"unexpected"}); err == nil {
		t.Fatal("unexpected argument error = nil")
	}
}
