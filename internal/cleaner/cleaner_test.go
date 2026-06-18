package cleaner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gocleaner/internal/model"
)

func testItem(path string, selected bool, risk string) model.ScanItem {
	return model.ScanItem{
		ID:       "test-file",
		Path:     path,
		Name:     filepath.Base(path),
		Type:     model.TypeFile,
		Category: model.CategorySystem,
		Size:     5,
		Risk:     risk,
		Source:   "test rule",
		Selected: selected,
	}
}

func writeTestFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return path
}

func TestCleanDeletesSelectedFileAndReportsFreedSize(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "delete.tmp", "hello")

	result, err := Clean([]model.ScanItem{testItem(path, true, model.RiskLow)}, Options{})
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if result.DeletedFiles != 1 {
		t.Fatalf("DeletedFiles = %d, want 1", result.DeletedFiles)
	}
	if result.FreedSize != 5 {
		t.Fatalf("FreedSize = %d, want 5", result.FreedSize)
	}
	if len(result.FailedFiles) != 0 {
		t.Fatalf("FailedFiles = %v, want empty", result.FailedFiles)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("deleted file still exists or stat failed unexpectedly: %v", statErr)
	}
}

func TestCleanDeduplicatesSelectedFilePaths(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "delete-once.tmp", "hello")
	first := testItem(path, true, model.RiskLow)
	second := first
	second.ID = "duplicate-id"
	second.Source = "duplicate rule"

	result, err := Clean([]model.ScanItem{first, second}, Options{})
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}
	if result.DeletedFiles != 1 {
		t.Fatalf("DeletedFiles = %d, want 1", result.DeletedFiles)
	}
	if len(result.FailedFiles) != 0 {
		t.Fatalf("duplicate path should not be cleaned twice, failures = %+v", result.FailedFiles)
	}
}

func TestCleanSkipsUnselectedItems(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "keep.tmp", "hello")

	result, err := Clean([]model.ScanItem{testItem(path, false, model.RiskLow)}, Options{})
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if result.DeletedFiles != 0 {
		t.Fatalf("DeletedFiles = %d, want 0", result.DeletedFiles)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("unselected file should remain: %v", statErr)
	}
}

func TestCleanRequiresHighRiskConfirmation(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "high-risk.tmp", "hello")

	result, err := Clean([]model.ScanItem{testItem(path, true, model.RiskHigh)}, Options{})
	if err == nil {
		t.Fatal("Clean error = nil, want high-risk confirmation error")
	}
	if result == nil {
		t.Fatal("Clean result = nil, want result with no deletion")
	}
	if result.DeletedFiles != 0 {
		t.Fatalf("DeletedFiles = %d, want 0", result.DeletedFiles)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("high-risk file should remain without confirmation: %v", statErr)
	}
}

func TestCleanDeletesHighRiskItemWhenConfirmed(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "confirmed.tmp", "hello")

	result, err := Clean([]model.ScanItem{testItem(path, true, model.RiskHigh)}, Options{
		HighRiskConfirmed: true,
	})
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}
	if result.DeletedFiles != 1 {
		t.Fatalf("DeletedFiles = %d, want 1", result.DeletedFiles)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("confirmed high-risk file still exists or stat failed unexpectedly: %v", statErr)
	}
}

func TestCleanRecordsMissingFileFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.tmp")

	result, err := Clean([]model.ScanItem{testItem(path, true, model.RiskLow)}, Options{})
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if result.DeletedFiles != 0 {
		t.Fatalf("DeletedFiles = %d, want 0", result.DeletedFiles)
	}
	if len(result.FailedFiles) != 1 || result.FailedFiles[0] != path {
		t.Fatalf("FailedFiles = %v, want [%s]", result.FailedFiles, path)
	}
	reason := strings.ToLower(result.FailedReasons[path])
	if !strings.Contains(reason, "not found") {
		t.Fatalf("failure reason = %q, want not found", result.FailedReasons[path])
	}
}

func TestCleanRejectsUnsupportedItemType(t *testing.T) {
	dir := t.TempDir()
	item := testItem(dir, true, model.RiskLow)
	item.Type = model.TypeDirectory

	result, err := Clean([]model.ScanItem{item}, Options{})
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if result.DeletedFiles != 0 {
		t.Fatalf("DeletedFiles = %d, want 0", result.DeletedFiles)
	}
	if len(result.FailedFiles) != 1 || result.FailedFiles[0] != dir {
		t.Fatalf("FailedFiles = %v, want [%s]", result.FailedFiles, dir)
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Fatalf("unsupported directory item should remain: %v", statErr)
	}
}
