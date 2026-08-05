package apitime

import (
	"testing"
	"time"
)

func TestFormatJSONTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    time.Time
		expected string
	}{
		{
			name:     "truncates to microseconds and preserves offset",
			value:    time.Date(2026, time.August, 4, 18, 37, 1, 4_990_999, time.FixedZone("EEST", 3*60*60)),
			expected: "2026-08-04T18:37:01.004990+03:00",
		},
		{
			name:     "uses six fractional digits for UTC",
			value:    time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC),
			expected: "2026-08-04T12:00:00.000000+00:00",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if actual := FormatJSONTime(test.value); actual != test.expected {
				t.Errorf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}
