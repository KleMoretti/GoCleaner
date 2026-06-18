package rules

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtectedNamesAreCaseInsensitive(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "lowercase browser sensitive pattern",
			json: `[{
				"name": "lower cookies",
				"category": "software",
				"paths": ["C:\\Cache"],
				"patterns": ["cookies"],
				"risk": "low",
				"default_on": true
			}]`,
		},
		{
			name: "lowercase im protected directory",
			json: `[{
				"name": "lower file dir",
				"category": "software",
				"paths": ["%USERPROFILE%\\Documents\\WeChat Files\\wxid_xxx\\file"],
				"patterns": ["*.dat"],
				"risk": "medium",
				"default_on": false
			}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempJSON(t, tt.json)
			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}
			if len(result.Rules) != 0 {
				t.Fatalf("case-insensitive protected target should be rejected, loaded %d rules", len(result.Rules))
			}
			if len(result.Errors) == 0 {
				t.Fatal("expected at least one validation error")
			}
		})
	}
}

func TestFirefoxSensitiveProfileTargetsRejected(t *testing.T) {
	tests := []string{
		`%APPDATA%\Mozilla\Firefox\Profiles\abc.default\places.sqlite`,
		`%APPDATA%\Mozilla\Firefox\Profiles\abc.default\cookies.sqlite`,
		`%APPDATA%\Mozilla\Firefox\Profiles\abc.default\logins.json`,
		`%APPDATA%\Mozilla\Firefox\Profiles\abc.default\key4.db`,
		`%APPDATA%\Mozilla\Firefox\Profiles\abc.default\prefs.js`,
	}

	for _, target := range tests {
		t.Run(filepath.Base(target), func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "Firefox sensitive",
				"category": "software",
				"paths": [%q],
				"patterns": [],
				"risk": "low",
				"default_on": true
			}]`, target)

			path := writeTempJSON(t, json)
			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}
			if len(result.Rules) != 0 {
				t.Fatalf("Firefox sensitive profile target %q should be rejected", target)
			}
			if len(result.Errors) == 0 {
				t.Fatal("expected at least one validation error")
			}
		})
	}
}

func TestExpandedProtectedSystemPathsAreRejected(t *testing.T) {
	t.Setenv("SystemRoot", `C:\\Windows`)
	t.Setenv("ProgramFiles", `C:\\Program Files`)

	tests := []string{
		`%SystemRoot%\\System32`,
		`%SystemRoot%\\System32\\drivers`,
		`%ProgramFiles%\\Vendor\\Cache`,
	}

	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "Expanded protected path",
				"category": "system",
				"paths": [%q],
				"patterns": ["*.tmp"],
				"risk": "high",
				"default_on": false
			}]`, target)

			result, err := LoadFromBytes([]byte(json))
			if err != nil {
				t.Fatalf("LoadFromBytes() unexpected error: %v", err)
			}
			if len(result.Rules) != 0 {
				t.Fatalf("expanded protected target %q should be rejected", target)
			}
			if len(result.Errors) == 0 {
				t.Fatal("expected validation error for expanded protected path")
			}
		})
	}
}

func TestRelativePathTargetsAreRejected(t *testing.T) {
	for _, target := range []string{`tmp`, `cache\\logs`, `..\\Temp`} {
		t.Run(target, func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "Relative target",
				"category": "system",
				"paths": [%q],
				"patterns": ["*.tmp"],
				"risk": "low",
				"default_on": true
			}]`, target)

			result, err := LoadFromBytes([]byte(json))
			if err != nil {
				t.Fatalf("LoadFromBytes() unexpected error: %v", err)
			}
			if len(result.Rules) != 0 {
				t.Fatalf("relative target %q should be rejected", target)
			}
			if len(result.Errors) == 0 {
				t.Fatal("expected validation error for relative path")
			}
		})
	}
}

func TestIMNonSafeDirectoryRejectedEvenWithPatterns(t *testing.T) {
	t.Setenv("USERPROFILE", `C:\\Users\\tester`)
	json := `[{
		"name": "WeChat user files",
		"category": "software",
		"paths": ["%USERPROFILE%\\\\Documents\\\\WeChat Files\\\\wxid_xxx\\\\FileStorage"],
		"patterns": ["*.tmp"],
		"exclude": ["Msg*.db"],
		"risk": "medium",
		"default_on": false
	}]`

	result, err := LoadFromBytes([]byte(json))
	if err != nil {
		t.Fatalf("LoadFromBytes() unexpected error: %v", err)
	}
	if len(result.Rules) != 0 {
		t.Fatalf("non-safe IM directory should be rejected even when patterns are specified")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected fatal validation error for non-safe IM directory")
	}
	if !strings.Contains(result.Errors[0].Message, "IM") {
		t.Fatalf("error should mention IM boundary, got %q", result.Errors[0].Message)
	}
}
