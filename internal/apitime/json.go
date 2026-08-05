// Package apitime formats timestamps for API JSON responses.
package apitime

import "time"

const jsonTimeLayout = "2006-01-02T15:04:05.000000-07:00"

// FormatJSONTime truncates a timestamp to microsecond precision and formats it
// with a numeric timezone offset for API JSON responses.
func FormatJSONTime(value time.Time) string {
	return value.Truncate(time.Microsecond).Format(jsonTimeLayout)
}
