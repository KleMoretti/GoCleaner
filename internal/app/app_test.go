package app

import (
	"os"
	"path/filepath"
	"strings"
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

func rememberAppTestItem(a *App, item model.ScanItem) {
	a.replaceAuthorizedItems([]model.ScanItem{item})
}

func TestAppCleanDeletesFileAndWritesOperationLog(t *testing.T) {
	dir := withTempWorkingDir(t)
	path := filepath.Join(dir, "delete.tmp")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	a := New(nil)
	item := appTestItem(path, true, model.RiskLow)
	rememberAppTestItem(a, item)
	result, err := a.Clean([]model.ScanItem{item}, false)
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

func TestAppCleanRejectsItemsNotProducedByLatestScan(t *testing.T) {
	dir := withTempWorkingDir(t)
	path := filepath.Join(dir, "forged.tmp")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	a := New(nil)
	result, err := a.Clean([]model.ScanItem{appTestItem(path, true, model.RiskLow)}, false)
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}
	if result.DeletedFiles != 0 {
		t.Fatalf("DeletedFiles = %d, want 0 for unscanned item", result.DeletedFiles)
	}
	if len(result.FailedFiles) != 1 || !strings.Contains(result.FailedReasons[path], "latest scan") {
		t.Fatalf("unauthorized clean result = %+v", result)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("unscanned file should remain: %v", statErr)
	}
}

func TestAppCleanUsesAuthorizedScanItemInsteadOfFrontendPath(t *testing.T) {
	dir := withTempWorkingDir(t)
	scannedPath := filepath.Join(dir, "scanned.tmp")
	forgedPath := filepath.Join(dir, "forged.tmp")
	if err := os.WriteFile(scannedPath, []byte("safe"), 0o600); err != nil {
		t.Fatalf("WriteFile scanned: %v", err)
	}
	if err := os.WriteFile(forgedPath, []byte("keep"), 0o600); err != nil {
		t.Fatalf("WriteFile forged: %v", err)
	}

	a := New(nil)
	authorized := appTestItem(scannedPath, false, model.RiskLow)
	rememberAppTestItem(a, authorized)

	request := authorized
	request.Path = forgedPath
	request.Selected = true
	result, err := a.Clean([]model.ScanItem{request}, false)
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}
	if result.DeletedFiles != 1 {
		t.Fatalf("DeletedFiles = %d, want 1 authorized deletion", result.DeletedFiles)
	}
	if _, statErr := os.Stat(scannedPath); !os.IsNotExist(statErr) {
		t.Fatalf("authorized scanned path should be deleted, stat err = %v", statErr)
	}
	if _, statErr := os.Stat(forgedPath); statErr != nil {
		t.Fatalf("forged frontend path should remain: %v", statErr)
	}
}

func TestAppCleanReturnsResultWhenOperationLogFailsAfterDeletion(t *testing.T) {
	dir := withTempWorkingDir(t)
	path := filepath.Join(dir, "delete-with-log-failure.tmp")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	a := New(nil)
	item := appTestItem(path, true, model.RiskLow)
	rememberAppTestItem(a, item)
	if err := os.WriteFile("data", []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile data sentinel: %v", err)
	}

	result, err := a.Clean([]model.ScanItem{item}, false)
	if err != nil {
		t.Fatalf("Clean should return result without rejecting after deletion; error = %v", err)
	}
	if result.DeletedFiles != 1 || len(result.Warnings) != 1 {
		t.Fatalf("Clean result = %+v, want deletion plus log warning", result)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("file should be deleted despite log failure, stat err = %v", statErr)
	}
}

func TestAppCleanHighRiskWithoutConfirmationDoesNotDelete(t *testing.T) {
	dir := withTempWorkingDir(t)
	path := filepath.Join(dir, "high-risk.tmp")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	a := New(nil)
	item := appTestItem(path, true, model.RiskHigh)
	rememberAppTestItem(a, item)
	result, err := a.Clean([]model.ScanItem{item}, false)
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

func TestAppShredFileWritesOperationLog(t *testing.T) {
	dir := withTempWorkingDir(t)
	path := filepath.Join(dir, "shred-me.txt")
	if err := os.WriteFile(path, []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	a := New(nil)
	result, err := a.ShredFile(model.ShredRequest{Path: path, Passes: 1}, true)
	if err != nil {
		t.Fatalf("ShredFile returned error: %v", err)
	}
	if result.ShreddedFiles != 1 {
		t.Fatalf("Shred result = %+v, want one shredded file", result)
	}

	logs, err := a.GetOperationLogs(10)
	if err != nil {
		t.Fatalf("GetOperationLogs returned error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("log count = %d, want 1", len(logs))
	}
	if logs[0].Operation != model.OpShred || logs[0].DeletedFiles != 1 || logs[0].FreedSize == 0 {
		t.Fatalf("operation log = %+v, want shred/deleted=1/freed>0", logs[0])
	}
}

func TestAppendRegistryActionLogRecordsBackupAndDeleteCounts(t *testing.T) {
	dir := withTempWorkingDir(t)
	_ = dir
	result := &model.RegistryActionResult{
		DeletedValues: 1,
		BackupPath:    filepath.Join("data", "registry_backup", "backup.reg"),
		FailedItems:   []string{`HKCU\Software\Test\Bad`},
		FailedReasons: map[string]string{`HKCU\Software\Test\Bad`: "permission denied"},
	}

	if err := appendRegistryActionLog(result, 12); err != nil {
		t.Fatalf("appendRegistryActionLog returned error: %v", err)
	}

	logs, err := New(nil).GetOperationLogs(10)
	if err != nil {
		t.Fatalf("GetOperationLogs returned error: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("log count = %d, want backup and delete logs", len(logs))
	}
	if logs[1].Operation != model.OpRegistryBackup || logs[1].DeletedFiles != 1 || len(logs[1].FailedPaths) != 0 {
		t.Fatalf("backup log = %+v", logs[1])
	}
	if logs[0].Operation != model.OpRegistryDelete || logs[0].DeletedFiles != 1 {
		t.Fatalf("delete log = %+v", logs[0])
	}
}
