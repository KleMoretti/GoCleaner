package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gocleaner/internal/model"
)

func writePluginManifest(t *testing.T, base, profile, extensionID, version, manifest string) string {
	t.Helper()
	versionDir := filepath.Join(base, profile, "Extensions", extensionID, version)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll version dir: %v", err)
	}
	manifestPath := filepath.Join(versionDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}
	return filepath.Join(base, profile, "Extensions", extensionID)
}

func writeLocaleMessage(t *testing.T, extensionRoot, locale string, body string) {
	t.Helper()
	localeDir := filepath.Join(extensionRoot, "_locales", locale)
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll locale dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localeDir, "messages.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile messages: %v", err)
	}
}

func TestScanBrowserPluginsReadsChromeManifestAndLocaleName(t *testing.T) {
	userData := filepath.Join(t.TempDir(), "Chrome", "User Data")
	extensionRoot := writePluginManifest(t, userData, "Default", "abcdefghijklmnop", "1.2.3", `{
		"name": "__MSG_extName__",
		"description": "__MSG_extDescription__",
		"version": "1.2.3",
		"default_locale": "en"
	}`)
	writeLocaleMessage(t, extensionRoot, "en", `{
		"extName": {"message": "Clean Test Extension"},
		"extDescription": {"message": "Scans manifest metadata"}
	}`)

	items, errs := ScanBrowserPlugins([]PluginTarget{{
		Browser:      "Chrome",
		UserDataPath: userData,
	}})

	if len(errs) != 0 {
		t.Fatalf("ScanBrowserPlugins errors = %+v, want none", errs)
	}
	if len(items) != 1 {
		t.Fatalf("plugin count = %d, want 1", len(items))
	}

	item := items[0]
	if item.Type != model.TypePlugin {
		t.Fatalf("item type = %q, want %q", item.Type, model.TypePlugin)
	}
	if item.Category != model.CategoryPlugin {
		t.Fatalf("category = %q, want %q", item.Category, model.CategoryPlugin)
	}
	if item.Risk != model.RiskMedium {
		t.Fatalf("risk = %q, want %q", item.Risk, model.RiskMedium)
	}
	if item.Selected {
		t.Fatal("plugin scan item must not be selected by default")
	}
	if item.Name != "Clean Test Extension" {
		t.Fatalf("name = %q, want localized manifest name", item.Name)
	}
	if item.Plugin == nil {
		t.Fatal("plugin metadata should be populated")
	}
	if item.Plugin.Browser != "Chrome" || item.Plugin.Profile != "Default" {
		t.Fatalf("plugin browser/profile = %+v, want Chrome/Default", item.Plugin)
	}
	if item.Plugin.ExtensionID != "abcdefghijklmnop" {
		t.Fatalf("extension id = %q", item.Plugin.ExtensionID)
	}
	if item.Plugin.Version != "1.2.3" {
		t.Fatalf("version = %q", item.Plugin.Version)
	}
	if item.Plugin.Description != "Scans manifest metadata" {
		t.Fatalf("description = %q", item.Plugin.Description)
	}
	if !strings.HasSuffix(item.Plugin.ManifestPath, filepath.Join("1.2.3", "manifest.json")) {
		t.Fatalf("manifest path = %q", item.Plugin.ManifestPath)
	}
	if item.Size <= 0 {
		t.Fatalf("plugin item size = %d, want positive directory size", item.Size)
	}
}

func TestScanBrowserPluginsUsesNewestVersionDirectory(t *testing.T) {
	userData := filepath.Join(t.TempDir(), "Edge", "User Data")
	writePluginManifest(t, userData, "Profile 1", "edgeextension", "1.0.0", `{"name":"Old","version":"1.0.0"}`)
	writePluginManifest(t, userData, "Profile 1", "edgeextension", "2.0.0", `{"name":"New","version":"2.0.0"}`)

	items, errs := ScanBrowserPlugins([]PluginTarget{{
		Browser:      "Edge",
		UserDataPath: userData,
	}})

	if len(errs) != 0 {
		t.Fatalf("ScanBrowserPlugins errors = %+v, want none", errs)
	}
	if len(items) != 1 {
		t.Fatalf("plugin count = %d, want 1", len(items))
	}
	if items[0].Name != "New" {
		t.Fatalf("name = %q, want New from latest version manifest", items[0].Name)
	}
	if items[0].Plugin == nil || items[0].Plugin.Version != "2.0.0" {
		t.Fatalf("plugin metadata = %+v, want version 2.0.0", items[0].Plugin)
	}
}

func TestScanBrowserPluginsRecordsBrokenManifestAndContinues(t *testing.T) {
	userData := filepath.Join(t.TempDir(), "Chrome", "User Data")
	writePluginManifest(t, userData, "Default", "brokenextension", "1.0.0", `{bad json`)
	writePluginManifest(t, userData, "Default", "goodextension", "1.0.0", `{"name":"Good","version":"1.0.0"}`)

	items, errs := ScanBrowserPlugins([]PluginTarget{{
		Browser:      "Chrome",
		UserDataPath: userData,
	}})

	if len(items) != 1 {
		t.Fatalf("plugin count = %d, want only good manifest", len(items))
	}
	if items[0].Name != "Good" {
		t.Fatalf("plugin name = %q, want Good", items[0].Name)
	}
	if len(errs) != 1 {
		t.Fatalf("error count = %d, want one broken manifest error", len(errs))
	}
	if !strings.Contains(strings.ToLower(errs[0].Reason), "manifest") {
		t.Fatalf("error reason = %q, want manifest parse reason", errs[0].Reason)
	}
}
