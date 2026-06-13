package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gocleaner/internal/model"
	"gocleaner/internal/windows"
)

func TestWriteRegistryBackupEscapesStringValues(t *testing.T) {
	item := model.ScanItem{
		Type:     model.TypeRegistry,
		Category: model.CategoryRegistry,
		Name:     "Bad Value",
		Registry: &model.RegistryInfo{
			Hive:      "HKCU",
			KeyPath:   RunKeyPath,
			ValueName: `Bad"Value`,
			ValueType: windows.RegistryString,
			RawData:   `C:\Program Files\Bad\App.exe`,
		},
	}
	path := filepath.Join(t.TempDir(), "backup.reg")

	if err := WriteRegistryBackup(path, []model.ScanItem{item}); err != nil {
		t.Fatalf("WriteRegistryBackup returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile backup: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "Windows Registry Editor Version 5.00") {
		t.Fatalf("backup missing reg header: %q", text)
	}
	if !strings.Contains(text, `[HKEY_CURRENT_USER\`+RunKeyPath+`]`) {
		t.Fatalf("backup missing HKCU Run key: %q", text)
	}
	if !strings.Contains(text, `"Bad\"Value"="C:\\Program Files\\Bad\\App.exe"`) {
		t.Fatalf("backup did not escape string value correctly: %q", text)
	}
}

func TestWriteRegistryBackupEncodesExpandStringValues(t *testing.T) {
	item := model.ScanItem{
		Type:     model.TypeRegistry,
		Category: model.CategoryRegistry,
		Name:     "Expand",
		Registry: &model.RegistryInfo{
			Hive:      "HKCU",
			KeyPath:   RunKeyPath,
			ValueName: "Expand",
			ValueType: windows.RegistryExpandString,
			RawData:   `%USERPROFILE%\App\app.exe`,
		},
	}
	path := filepath.Join(t.TempDir(), "backup.reg")

	if err := WriteRegistryBackup(path, []model.ScanItem{item}); err != nil {
		t.Fatalf("WriteRegistryBackup returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile backup: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, `"Expand"=hex(2):`) {
		t.Fatalf("REG_EXPAND_SZ should be written as hex(2): %q", text)
	}
	if !strings.Contains(text, "25,00,55,00,53,00,45,00,52,00") {
		t.Fatalf("hex(2) data should contain UTF-16LE bytes for %%USER: %q", text)
	}
}

func TestDeleteRegistryItemsWithBackupFailureStopsBeforeDelete(t *testing.T) {
	backupDirFile := filepath.Join(t.TempDir(), "backup-dir-is-file")
	if err := os.WriteFile(backupDirFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile backup dir sentinel: %v", err)
	}

	item := model.ScanItem{
		ID:       "registry_bad",
		Path:     `HKCU\` + RunKeyPath + `\Bad`,
		Type:     model.TypeRegistry,
		Category: model.CategoryRegistry,
		Risk:     model.RiskHigh,
		Selected: true,
		Registry: &model.RegistryInfo{
			Hive:      "HKCU",
			KeyPath:   RunKeyPath,
			ValueName: "Bad",
			ValueType: windows.RegistryString,
			RawData:   `C:\Missing\Bad.exe`,
		},
	}

	result, err := DeleteRegistryItemsWithBackupDir([]model.ScanItem{item}, true, backupDirFile)
	if err != nil {
		t.Fatalf("DeleteRegistryItemsWithBackupDir returned error: %v", err)
	}
	if result.DeletedValues != 0 {
		t.Fatalf("DeletedValues = %d, want 0 when backup fails", result.DeletedValues)
	}
	if result.BackupPath != "" {
		t.Fatalf("BackupPath = %q, want empty on failed backup", result.BackupPath)
	}
	if len(result.FailedItems) != 1 || !strings.Contains(result.FailedReasons[item.Path], "registry backup failed") {
		t.Fatalf("backup failure result = %+v", result)
	}
}
