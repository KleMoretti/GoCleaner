package cleaner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gocleaner/internal/model"
)

func pluginItem(path string) model.ScanItem {
	return model.ScanItem{
		ID:       "plugin-test",
		Path:     path,
		Name:     "Test Plugin",
		Type:     model.TypePlugin,
		Category: model.CategoryPlugin,
		Risk:     model.RiskMedium,
		Selected: true,
		Plugin: &model.PluginInfo{
			Browser:     "Chrome",
			Profile:     "Default",
			ExtensionID: "testplugin",
			Version:     "1.0.0",
		},
	}
}

func makePluginDir(t *testing.T, root string) string {
	t.Helper()
	pluginPath := filepath.Join(root, "Chrome", "User Data", "Default", "Extensions", "testplugin")
	versionPath := filepath.Join(pluginPath, "1.0.0")
	if err := os.MkdirAll(versionPath, 0o755); err != nil {
		t.Fatalf("MkdirAll plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionPath, "manifest.json"), []byte(`{"name":"Test Plugin","version":"1.0.0"}`), 0o600); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}
	return pluginPath
}

func TestQuarantinePluginsMovesSelectedPluginAndRestoreReturnsIt(t *testing.T) {
	workDir := t.TempDir()
	pluginPath := makePluginDir(t, workDir)
	quarantineRoot := filepath.Join(workDir, "data", "quarantine", "plugins")
	store := NewQuarantineStore(quarantineRoot)

	result, err := store.QuarantinePlugins([]model.ScanItem{pluginItem(pluginPath)})
	if err != nil {
		t.Fatalf("QuarantinePlugins returned error: %v", err)
	}
	if result.MovedItems != 1 {
		t.Fatalf("MovedItems = %d, want 1", result.MovedItems)
	}
	if len(result.FailedItems) != 0 {
		t.Fatalf("FailedItems = %+v, want none", result.FailedItems)
	}
	if _, statErr := os.Stat(pluginPath); !os.IsNotExist(statErr) {
		t.Fatalf("original plugin path should be moved away, stat err = %v", statErr)
	}

	records, err := store.ListRecords()
	if err != nil {
		t.Fatalf("ListRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	if records[0].OriginalPath != pluginPath {
		t.Fatalf("OriginalPath = %q, want %q", records[0].OriginalPath, pluginPath)
	}
	if records[0].Browser != "Chrome" {
		t.Fatalf("Browser = %q, want Chrome", records[0].Browser)
	}
	if records[0].Size <= 0 {
		t.Fatalf("record size = %d, want positive", records[0].Size)
	}

	restore, err := store.RestorePlugin(records[0].RecordID)
	if err != nil {
		t.Fatalf("RestorePlugin returned error: %v", err)
	}
	if restore.RestoredItems != 1 {
		t.Fatalf("RestoredItems = %d, want 1", restore.RestoredItems)
	}
	if _, statErr := os.Stat(filepath.Join(pluginPath, "1.0.0", "manifest.json")); statErr != nil {
		t.Fatalf("restored manifest missing: %v", statErr)
	}
}

func TestQuarantinePluginsFallsBackWhenRenameFails(t *testing.T) {
	workDir := t.TempDir()
	pluginPath := makePluginDir(t, workDir)
	store := NewQuarantineStore(filepath.Join(workDir, "data", "quarantine", "plugins"))

	originalRename := renamePath
	renamePath = func(oldPath, newPath string) error {
		return errors.New("simulated cross-volume rename failure")
	}
	t.Cleanup(func() {
		renamePath = originalRename
	})

	result, err := store.QuarantinePlugins([]model.ScanItem{pluginItem(pluginPath)})
	if err != nil {
		t.Fatalf("QuarantinePlugins returned error: %v", err)
	}
	if result.MovedItems != 1 {
		t.Fatalf("MovedItems = %d, want 1 with copy/remove fallback", result.MovedItems)
	}
	if _, statErr := os.Stat(pluginPath); !os.IsNotExist(statErr) {
		t.Fatalf("original plugin path should be removed after fallback move, stat err = %v", statErr)
	}
	records, err := store.ListRecords()
	if err != nil {
		t.Fatalf("ListRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	if _, statErr := os.Stat(filepath.Join(records[0].QuarantinePath, "1.0.0", "manifest.json")); statErr != nil {
		t.Fatalf("quarantined manifest missing after fallback move: %v", statErr)
	}
}

func TestListRecordsHandlesLargeJSONLine(t *testing.T) {
	store := NewQuarantineStore(filepath.Join(t.TempDir(), "quarantine"))
	record := model.QuarantineRecord{
		RecordID:       "large-record",
		OriginalPath:   strings.Repeat("C", 70*1024),
		QuarantinePath: filepath.Join(t.TempDir(), "content"),
		Name:           "Large",
		ItemType:       model.TypePlugin,
		CreatedAt:      "2026-06-18T10:00:00+08:00",
	}
	if err := store.appendRecord(record); err != nil {
		t.Fatalf("appendRecord returned error: %v", err)
	}

	records, err := store.ListRecords()
	if err != nil {
		t.Fatalf("ListRecords returned error for large JSONL row: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	if len(records[0].OriginalPath) != 70*1024 {
		t.Fatalf("large original path was not preserved")
	}
}

func TestQuarantinePluginsDoesNotMoveWhenRecordWriteFails(t *testing.T) {
	workDir := t.TempDir()
	pluginPath := makePluginDir(t, workDir)
	quarantineRoot := filepath.Join(workDir, "data", "quarantine", "plugins")
	if err := os.MkdirAll(filepath.Join(quarantineRoot, "index.jsonl"), 0o755); err != nil {
		t.Fatalf("MkdirAll index sentinel: %v", err)
	}
	store := NewQuarantineStore(quarantineRoot)

	result, err := store.QuarantinePlugins([]model.ScanItem{pluginItem(pluginPath)})
	if err != nil {
		t.Fatalf("QuarantinePlugins returned error: %v", err)
	}
	if result.MovedItems != 0 || len(result.FailedItems) != 1 {
		t.Fatalf("Quarantine result = %+v, want record failure", result)
	}
	if _, statErr := os.Stat(filepath.Join(pluginPath, "1.0.0", "manifest.json")); statErr != nil {
		t.Fatalf("plugin should remain at original path after record failure: %v", statErr)
	}
	records, recordsErr := store.ListRecords()
	if recordsErr == nil && len(records) != 0 {
		t.Fatalf("records = %+v, want none after failed quarantine", records)
	}
}

func TestQuarantinePluginsRejectsNonPluginItem(t *testing.T) {
	workDir := t.TempDir()
	filePath := writeTestFile(t, workDir, "cache.tmp", "hello")
	store := NewQuarantineStore(filepath.Join(workDir, "quarantine"))
	item := testItem(filePath, true, model.RiskLow)

	result, err := store.QuarantinePlugins([]model.ScanItem{item})
	if err != nil {
		t.Fatalf("QuarantinePlugins returned error: %v", err)
	}
	if result.MovedItems != 0 {
		t.Fatalf("MovedItems = %d, want 0", result.MovedItems)
	}
	if len(result.FailedItems) != 1 {
		t.Fatalf("FailedItems = %+v, want one failure", result.FailedItems)
	}
	if !strings.Contains(result.FailedReasons[filePath], "plugin") {
		t.Fatalf("failure reason = %q, want plugin type rejection", result.FailedReasons[filePath])
	}
	if _, statErr := os.Stat(filePath); statErr != nil {
		t.Fatalf("non-plugin file should remain: %v", statErr)
	}
}

func TestQuarantinePluginsRejectsPluginItemOutsideExtensionsRoot(t *testing.T) {
	workDir := t.TempDir()
	notExtensionPath := filepath.Join(workDir, "Documents", "important-folder")
	if err := os.MkdirAll(notExtensionPath, 0o755); err != nil {
		t.Fatalf("MkdirAll non-extension dir: %v", err)
	}
	store := NewQuarantineStore(filepath.Join(workDir, "quarantine"))

	result, err := store.QuarantinePlugins([]model.ScanItem{pluginItem(notExtensionPath)})
	if err != nil {
		t.Fatalf("QuarantinePlugins returned error: %v", err)
	}
	if result.MovedItems != 0 {
		t.Fatalf("MovedItems = %d, want 0", result.MovedItems)
	}
	if len(result.FailedItems) != 1 {
		t.Fatalf("FailedItems = %+v, want one failure", result.FailedItems)
	}
	if !strings.Contains(strings.ToLower(result.FailedReasons[notExtensionPath]), "extensions") {
		t.Fatalf("failure reason = %q, want extension root rejection", result.FailedReasons[notExtensionPath])
	}
	if _, statErr := os.Stat(notExtensionPath); statErr != nil {
		t.Fatalf("non-extension plugin item should remain: %v", statErr)
	}
}

func TestCleanStillRejectsPluginItems(t *testing.T) {
	workDir := t.TempDir()
	pluginPath := makePluginDir(t, workDir)

	result, err := Clean([]model.ScanItem{pluginItem(pluginPath)}, Options{})
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}
	if result.DeletedFiles != 0 {
		t.Fatalf("DeletedFiles = %d, want 0", result.DeletedFiles)
	}
	if len(result.FailedFiles) != 1 {
		t.Fatalf("FailedFiles = %+v, want plugin rejection", result.FailedFiles)
	}
	if _, statErr := os.Stat(pluginPath); statErr != nil {
		t.Fatalf("plugin dir should remain after normal clean: %v", statErr)
	}
}
