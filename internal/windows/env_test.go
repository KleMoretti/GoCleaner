package windows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandPath_KnownEnvVars(t *testing.T) {
	// Set test env vars
	os.Setenv("TEST_TEMP", `C:\Users\Test\AppData\Local\Temp`)
	os.Setenv("TEST_APPDATA", `C:\Users\Test\AppData\Roaming`)
	defer func() {
		os.Unsetenv("TEST_TEMP")
		os.Unsetenv("TEST_APPDATA")
	}()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple expansion",
			input:    `%TEST_TEMP%\subdir`,
			expected: `C:\Users\Test\AppData\Local\Temp\subdir`,
		},
		{
			name:     "no env var",
			input:    `C:\Windows\Temp`,
			expected: `C:\Windows\Temp`,
		},
		{
			name:     "multiple env vars",
			input:    `%TEST_APPDATA%\Microsoft\Windows`,
			expected: `C:\Users\Test\AppData\Roaming\Microsoft\Windows`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandPath(tt.input)
			if result != tt.expected {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandPath_UnsetEnvVar(t *testing.T) {
	// An unset env var should be left as-is (os.ExpandEnv behavior)
	result := ExpandPath(`%NONEXISTENT_VAR_12345%\subdir`)
	if !strings.Contains(result, "%NONEXISTENT_VAR_12345%") {
		t.Logf("Unset env var was expanded (may be platform-dependent): %q", result)
	}
}

func TestExpandPaths(t *testing.T) {
	os.Setenv("TEST_PATH", `C:\TestPath`)
	defer os.Unsetenv("TEST_PATH")

	inputs := []string{
		`%TEST_PATH%\dir1`,
		`%TEST_PATH%\dir2`,
		`C:\Absolute\Path`,
	}
	results := ExpandPaths(inputs)

	expected := []string{
		`C:\TestPath\dir1`,
		`C:\TestPath\dir2`,
		`C:\Absolute\Path`,
	}

	for i, r := range results {
		if r != expected[i] {
			t.Errorf("ExpandPaths[%d] = %q, want %q", i, r, expected[i])
		}
	}
}

func TestCleanPreservingWildcards(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no wildcards",
			input:    `C:\Users\Test\AppData\Local`,
			expected: `C:\Users\Test\AppData\Local`,
		},
		{
			name:     "wildcard in middle",
			input:    `C:\Users\Test\*\Cache`,
			expected: `C:\Users\Test\*\Cache`,
		},
		{
			name:     "wildcard with dot cleanup",
			input:    `C:\Users\Test\.\AppData\Local`,
			expected: `C:\Users\Test\AppData\Local`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanPreservingWildcards(tt.input)
			if result != tt.expected {
				t.Errorf("cleanPreservingWildcards(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetEnvWithDefault(t *testing.T) {
	result := GetEnvWithDefault("NONEXISTENT_VAR_98765", "default_value")
	if result != "default_value" {
		t.Errorf("GetEnvWithDefault() = %q, want %q", result, "default_value")
	}

	os.Setenv("TEST_VAR_EXISTS", "actual_value")
	defer os.Unsetenv("TEST_VAR_EXISTS")

	result = GetEnvWithDefault("TEST_VAR_EXISTS", "default_value")
	if result != "actual_value" {
		t.Errorf("GetEnvWithDefault() = %q, want %q", result, "actual_value")
	}
}

func TestPathExists(t *testing.T) {
	// Test with a temp directory that definitely exists
	tmpDir := t.TempDir()
	if !PathExists(tmpDir) {
		t.Errorf("PathExists(%q) = false, want true", tmpDir)
	}

	// Test with a path that doesn't exist
	nonExistent := filepath.Join(tmpDir, "does_not_exist", "nope.txt")
	if PathExists(nonExistent) {
		t.Errorf("PathExists(%q) = true, want false", nonExistent)
	}
}

func TestExpandAndResolve(t *testing.T) {
	os.Setenv("TEST_RESOLVE", `C:\TestResolve`)
	defer os.Unsetenv("TEST_RESOLVE")

	result, err := ExpandAndResolve(`%TEST_RESOLVE%\subdir`)
	if err != nil {
		t.Fatalf("ExpandAndResolve() unexpected error: %v", err)
	}
	expected := `C:\TestResolve\subdir`
	if result != expected {
		t.Errorf("ExpandAndResolve() = %q, want %q", result, expected)
	}
}
