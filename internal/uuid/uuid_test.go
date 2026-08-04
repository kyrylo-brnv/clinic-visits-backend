package uuid

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:  "valid",
			value: "11111111-1111-1111-1111-111111111111",
		},
		{
			name:    "empty",
			value:   "",
			wantErr: true,
		},
		{
			name:    "invalid hex",
			value:   "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz",
			wantErr: true,
		},
		{
			name:    "missing separators",
			value:   "111111111111111111111111111111111111",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			id, err := Parse(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id.String() != test.value {
				t.Fatalf("expected UUID %q, got %q", test.value, id.String())
			}
		})
	}
}

func TestIsValidOptional(t *testing.T) {
	t.Parallel()

	if !IsValidOptional("") {
		t.Fatal("expected empty UUID to be valid when optional")
	}
	if !IsValidOptional("11111111-1111-1111-1111-111111111111") {
		t.Fatal("expected UUID to be valid when optional")
	}
	if IsValidOptional("invalid") {
		t.Fatal("expected invalid UUID to be rejected")
	}
}
