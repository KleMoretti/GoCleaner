package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gocleaner/internal/model"
	"gocleaner/internal/windows"
)

func TestParseStartupCommandExtractsExecutablePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)

	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{
			name: "quoted path with arguments",
			raw:  `"C:\Program Files\App\app.exe" --silent`,
			want: `C:\Program Files\App\app.exe`,
			ok:   true,
		},
		{
			name: "environment path with arguments",
			raw:  `%USERPROFILE%\Missing\app.exe /start`,
			want: filepath.Join(home, "Missing", "app.exe"),
			ok:   true,
		},
		{
			name: "relative command is ignored",
			raw:  `OneDrive.exe /background`,
			ok:   false,
		},
		{
			name: "system command is ignored",
			raw:  `rundll32.exe shell32.dll,Control_RunDLL`,
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseStartupCommand(tt.raw)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("path = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildInvalidStartupItemsOnlyReportsMissingAbsoluteTargets(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.exe")
	existingDir := t.TempDir()
	existing := filepath.Join(existingDir, "exists.exe")
	if err := os.WriteFile(existing, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile existing: %v", err)
	}

	values := []windows.RegistryValue{
		{Name: "Missing", Type: windows.RegistryString, Data: `"` + missing + `" --boot`},
		{Name: "Existing", Type: windows.RegistryString, Data: `"` + existing + `"`},
		{Name: "Relative", Type: windows.RegistryString, Data: `Updater.exe /background`},
		{Name: "Dword", Type: windows.RegistryDWord, Data: `1`},
	}

	items := BuildInvalidStartupItems(values, fileExists)

	if len(items) != 1 {
		t.Fatalf("invalid item count = %d, want 1 (%+v)", len(items), items)
	}
	item := items[0]
	if item.Type != model.TypeRegistry || item.Category != model.CategoryRegistry {
		t.Fatalf("item type/category = %s/%s, want registry/registry", item.Type, item.Category)
	}
	if item.Risk != model.RiskHigh || item.Selected {
		t.Fatalf("registry item risk/selected = %s/%v, want high/false", item.Risk, item.Selected)
	}
	if item.Registry == nil {
		t.Fatal("registry metadata should be populated")
	}
	if item.Registry.ValueName != "Missing" {
		t.Fatalf("value name = %q, want Missing", item.Registry.ValueName)
	}
	if item.Registry.TargetPath != missing {
		t.Fatalf("target path = %q, want %q", item.Registry.TargetPath, missing)
	}
	if !strings.Contains(item.Path, RunKeyPath) {
		t.Fatalf("item path = %q, should contain Run key", item.Path)
	}
}
