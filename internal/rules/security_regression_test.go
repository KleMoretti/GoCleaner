package rules

import (
	"fmt"
	"path/filepath"
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
