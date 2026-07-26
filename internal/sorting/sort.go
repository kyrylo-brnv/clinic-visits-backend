package sorting

type Direction string

const (
	Ascending  Direction = "asc"
	Descending Direction = "desc"
)

type Sort struct {
	Field     string    `json:"field"`
	Direction Direction `json:"direction"`
}

type Config struct {
	allowedFields map[string]struct{}
}

func NewAllowedFields(fields ...string) Config {
	allowedFields := make(map[string]struct{}, len(fields))

	for _, field := range fields {
		allowedFields[field] = struct{}{}
	}

	return Config{
		allowedFields: allowedFields,
	}
}

func (c Config) IsValid(sort *Sort) bool {
	if sort == nil {
		return true
	}

	if _, ok := c.allowedFields[sort.Field]; !ok {
		return false
	}

	return sort.Direction == Ascending || sort.Direction == Descending
}
