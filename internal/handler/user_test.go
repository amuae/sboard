package handler

import "testing"

func TestValidateExpiryDate(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{value: "2026-08-01", valid: true},
		{value: "2026-02-30", valid: false},
		{value: "2026/08/01", valid: false},
		{value: "", valid: false},
	} {
		if got := validateExpiryDate(test.value) == nil; got != test.valid {
			t.Errorf("validateExpiryDate(%q) = %v, want %v", test.value, got, test.valid)
		}
	}
}

func TestValidateEnabled(t *testing.T) {
	valid := 1
	invalid := 2
	if err := validateEnabled(nil); err != nil {
		t.Fatalf("validateEnabled(nil) error = %v", err)
	}
	if err := validateEnabled(&valid); err != nil {
		t.Fatalf("validateEnabled(1) error = %v", err)
	}
	if err := validateEnabled(&invalid); err == nil {
		t.Fatal("validateEnabled(2) should return an error")
	}
}
