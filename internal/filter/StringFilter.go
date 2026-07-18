package filter

type StringFilter struct {
	Equals    *string `json:"equals,omitempty"`
	NotEquals *string `json:"not_equals,omitempty"`
}

func (f *StringFilter) IsEmpty() bool {
	return f == nil || (f.Equals == nil && f.NotEquals == nil)
}
