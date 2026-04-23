package validation

import (
	"testing"
)

func TestValidatePeerName_Valid(t *testing.T) {
	validNames := []string{
		"alice",
		"bob-smith",
		"charlie_123",
		"a",
		"Z9_-x",
		"User_Name-01",
	}
	for _, name := range validNames {
		err := ValidatePeerName(name)
		if err != nil {
			t.Errorf("ValidatePeerName(%q) returned error: %v, want nil", name, err)
		}
	}
}

func TestValidatePeerName_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "", wantErr: true},
		{name: "   ", wantErr: true}, // whitespace only
		{name: " alice", wantErr: true},          // leading space
		{name: "alice ", wantErr: true},          // trailing space
		{name: "a..b", wantErr: true},            // path traversal
		{name: "../etc/passwd", wantErr: true},   // path traversal
		{name: "a/b", wantErr: true},             // slash
		{name: "a\\b", wantErr: true},            // backslash
		{name: "alice@example.com", wantErr: true}, // @
		{name: "bob#smith", wantErr: true},       // hash
		{name: "charlie$money", wantErr: true},   // dollar
		{name: "dave&alice", wantErr: true},      // ampersand
		{name: "eve<script>", wantErr: true},    // angle brackets
		{name: "f\name", wantErr: true},          // newline control
		{name: "a very long name that definitely exceeds the sixty-four character limit by a significant margin and should be rejected", wantErr: true},
	}
	for _, tt := range tests {
		err := ValidatePeerName(tt.name)
		if err == nil && tt.wantErr {
			t.Errorf("ValidatePeerName(%q) expected error but got nil", tt.name)
		}
		if err != nil && !tt.wantErr {
			t.Errorf("ValidatePeerName(%q) unexpected error: %v", tt.name, err)
		}
	}
}

func TestValidatePeerName_LengthBoundaries(t *testing.T) {
	// 1 character should be ok
	err := ValidatePeerName("a")
	if err != nil {
		t.Errorf("ValidatePeerName('a') = %v, want nil", err)
	}
	// 64 characters should be ok (all 'a's)
	long64 := strings.Repeat("a", 64)
	err = ValidatePeerName(long64)
	if err != nil {
		t.Errorf("ValidatePeerName(64 'a's) = %v, want nil", err)
	}
	// 65 characters should fail
	long65 := strings.Repeat("a", 65)
	err = ValidatePeerName(long65)
	if err == nil {
		t.Errorf("ValidatePeerName(65 'a's) expected error, got nil")
	}
}
