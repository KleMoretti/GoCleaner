package app

import (
	"os"
	"path/filepath"
	"testing"

	"gocleaner/internal/model"
)

func withTempWorkingDir(t *testing.T) string {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir temp: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	})
	return dir
}

func appTestItem(path string, selected bool, risk string) model.ScanItem {
	return model.ScanItem{
		ID:       "app-test-file",
		Path:     path,
		Name:     filepath.Base(path),
		Type:     model.TypeFile,
		Category: model.CategorySystem,
		Size:     5,
		Risk:     risk,
		Source:   "app test rule",
		Selected: selected,
	}
}

func TestAppCleanDeletesFileAndWritesOperationLog(t *testing.T) {
	dir := withTempWorkingDir(t)
	path := filepath.Join(dir, "delete.tmp")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	a := New(nil)
	result, err := a.Clean([]model.ScanItem{appTestItem(path, true, model.RiskLow)}, false)
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}
	if result.DeletedFiles != 1 || result.FreedSize != 5 {
		t.Fatalf("Clean result = %+v, want 1 deleted and 5 freed", result)
	}

	logs, err := a.GetOperationLogs(10)
	if err != nil {
		t.Fatalf("GetOperationLogs returned error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("log count = %d, want 1", len(logs))
	}
	if logs[0].Operation != model.OpClean || logs[0].DeletedFiles != 1 || logs[0].FreedSize != 5 {
		t.Fatalf("operation log = %+v, want clean/deleted=1/freed=5", logs[0])
	}
}

func TestAppCleanHighRiskWithoutConfirmationDoesNotDelete(t *testing.T) {
	dir := withTempWorkingDir(t)
	path := filepath.Join(dir, "high-risk.tmp")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	a := New(nil)
	result, err := a.Clean([]model.ScanItem{appTestItem(path, true, model.RiskHigh)}, false)
	if err == nil {
		t.Fatal("Clean error = nil, want high-risk confirmation error")
	}
	if result == nil || result.DeletedFiles != 0 {
		t.Fatalf("Clean result = %+v, want no deletion", result)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("high-risk file should remain without confirmation: %v", statErr)
	}
}
