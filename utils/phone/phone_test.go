package phone

import "testing"

func TestNormalize_Valid(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"20123456", "+21620123456"},
		{"50 123 456", "+21650123456"},
		{"+216 20 123 456", "+21620123456"},
		{"+21620123456", "+21620123456"},
		{"0021620123456", "+21620123456"},
		{"21620123456", "+21620123456"},
		{"20-123-456", "+21620123456"},
		{"  90123456  ", "+21690123456"},
		{"71234567", "+21671234567"}, // landline (Tunis)
	}
	for _, tc := range cases {
		got, err := Normalize(tc.in)
		if err != nil {
			t.Errorf("Normalize(%q) unexpected err: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalize_Invalid(t *testing.T) {
	invalid := []string{
		"",                // empty
		"   ",             // blank
		"1234567",         // too short
		"123456789",       // too long (9 digits, no country)
		"60123456",        // invalid prefix 6
		"80123456",        // invalid prefix 8
		"10123456",        // invalid prefix 1
		"+33612345678",    // non-TN country code
		"abcdefgh",        // not digits
		"+216 2012345",    // 7 local digits only
		"+216201234567",   // 9 local digits
		"00216 6012 3456", // invalid prefix with country code
	}
	for _, in := range invalid {
		if _, err := Normalize(in); err == nil {
			t.Errorf("Normalize(%q) expected error, got nil", in)
		}
	}
}

func TestIsValid(t *testing.T) {
	if !IsValid("20123456") {
		t.Error("expected 20123456 to be valid")
	}
	if IsValid("60123456") {
		t.Error("expected 60123456 to be invalid")
	}
}
