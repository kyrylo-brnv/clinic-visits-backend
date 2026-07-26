package filter

type StringFilter struct {
	Equals    *string `json:"equals,omitempty"`
	NotEquals *string `json:"not_equals,omitempty"`
}

func (f *StringFilter) IsEmpty() bool {
	if f == nil {
		return true
	}

	equalsEmpty := f.Equals == nil || *f.Equals == ""
	notEqualsEmpty := f.NotEquals == nil || *f.NotEquals == ""

	return equalsEmpty && notEqualsEmpty
}

func (f *StringFilter) HasEquals() bool {
	return !f.IsEmpty() && f.isValidValue(f.Equals)
}

func (f *StringFilter) HasNotEquals() bool {
	return !f.IsEmpty() && f.isValidValue(f.NotEquals)
}

func (f *StringFilter) isValidValue(value *string) bool {
	return value != nil && *value != ""
}
