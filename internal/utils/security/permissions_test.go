package security

import (
	"os"
	"testing"
)

func TestCheckFilePermissions(t *testing.T) {
	// Create a temporary file with various permissions to test
	tmpFile, err := os.CreateTemp("", "test-perms-*.env")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name        string
		perm        os.FileMode
		strict      bool
		shouldFail  bool
		errorSubstr string
	}{
		{
			name:        "0600 strict - secure",
			perm:        0600,
			strict:      true,
			shouldFail:  false,
			errorSubstr: "",
		},
		{
			name:        "0600 non-strict - secure",
			perm:        0600,
			strict:      false,
			shouldFail:  false,
			errorSubstr: "",
		},
		{
			name:        "0644 strict - insecure world-readable",
			perm:        0644,
			strict:      true,
			shouldFail:  true,
			errorSubstr: "insecure permissions",
		},
		{
			name:        "0644 non-strict - insecure but not world-readable? Actually 0644 has world-read (004) so should fail",
			perm:        0644,
			strict:      false,
			shouldFail:  true,
			errorSubstr: "group/other permissions",
		},
		{
			name:        "0660 strict - group writable",
			perm:        0660,
			strict:      true,
			shouldFail:  true,
			errorSubstr: "insecure permissions",
		},
		{
			name:        "0660 non-strict - group writable allowed? No, group-write is also insecure",
			perm:        0660,
			strict:      false,
			shouldFail:  true,
			errorSubstr: "group/other permissions",
		},
		{
			name:        "0700 strict - secure (owner only)",
			perm:        0700,
			strict:      true,
			shouldFail:  false,
			errorSubstr: "",
		},
		{
			name:        "0400 strict - read-only secure",
			perm:        0400,
			strict:      true,
			shouldFail:  false,
			errorSubstr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set file permissions
			if err := os.Chmod(tmpFile.Name(), tt.perm); err != nil {
				t.Fatalf("Failed to chmod: %v", err)
			}

			err := CheckFilePermissions(tmpFile.Name(), tt.strict)
			failed := err != nil

			if failed != tt.shouldFail {
				if tt.shouldFail {
					t.Errorf("Expected error for perm %04o strict=%v, but got none: %v", tt.perm, tt.strict, err)
				} else {
					t.Errorf("Expected no error for perm %04o strict=%v, but got: %v", tt.perm, tt.strict, err)
				}
			}

			// If we expected an error, check the message
			if tt.shouldFail && err != nil && tt.errorSubstr != "" {
				if !contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Error message should contain %q, got %q", tt.errorSubstr, err.Error())
				}
			}
		})
	}
}

func TestIsWorldReadable(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-worldread-*.env")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set world-readable (0644)
	if err := os.Chmod(tmpFile.Name(), 0644); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}

	worldReadable, err := IsWorldReadable(tmpFile.Name())
	if err != nil {
		t.Fatalf("IsWorldReadable error: %v", err)
	}
	if !worldReadable {
		t.Error("Expected file with 0644 to be world-readable")
	}

	// Set to 0600 (not world-readable)
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}
	worldReadable, err = IsWorldReadable(tmpFile.Name())
	if err != nil {
		t.Fatalf("IsWorldReadable error: %v", err)
	}
	if worldReadable {
		t.Error("Expected file with 0600 to NOT be world-readable")
	}
}

func TestIsWorldWritable(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-worldwrite-*.env")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set world-writable (0666)
	if err := os.Chmod(tmpFile.Name(), 0666); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}

	worldWritable, err := IsWorldWritable(tmpFile.Name())
	if err != nil {
		t.Fatalf("IsWorldWritable error: %v", err)
	}
	if !worldWritable {
		t.Error("Expected file with 0666 to be world-writable")
	}

	// Set to 0600 (not world-writable)
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}
	worldWritable, err = IsWorldWritable(tmpFile.Name())
	if err != nil {
		t.Fatalf("IsWorldWritable error: %v", err)
	}
	if worldWritable {
		t.Error("Expected file with 0600 to NOT be world-writable")
	}
}

func TestCheckEnvFilePermissions_NonExistent(t *testing.T) {
	// Non-existent file should not error
	err := CheckEnvFilePermissions("/nonexistent/path/.env", true)
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got: %v", err)
	}
}

func TestSecureFileMode(t *testing.T) {
	mode := SecureFileMode()
	if mode != 0600 {
		t.Errorf("SecureFileMode() = %04o, want 0600", mode)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
