package registry

import (
	"strings"
	"testing"

	"gocleaner/internal/model"
)

func assertChineseRegistryMessage(t *testing.T, value string, banned ...string) {
	t.Helper()
	if strings.TrimSpace(value) == "" {
		t.Fatal("registry message is empty")
	}
	for _, phrase := range banned {
		if strings.Contains(strings.ToLower(value), strings.ToLower(phrase)) {
			t.Fatalf("registry message %q should not contain English phrase %q", value, phrase)
		}
	}
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			return
		}
	}
	t.Fatalf("registry message %q should contain Chinese text", value)
}

func TestRegistryRuntimeMessagesAreChinese(t *testing.T) {
	result, err := DeleteRegistryItemsWithBackupDir(nil, false, t.TempDir())
	if err != nil {
		t.Fatalf("DeleteRegistryItemsWithBackupDir returned error: %v", err)
	}
	assertChineseRegistryMessage(t, result.Message, "No registry values selected")

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
		},
	}
	result, err = DeleteRegistryItemsWithBackupDir([]model.ScanItem{item}, false, t.TempDir())
	if err == nil {
		t.Fatal("DeleteRegistryItemsWithBackupDir error = nil, want confirmation error")
	}
	assertChineseRegistryMessage(t, err.Error(), "registry delete confirmation required")
	assertChineseRegistryMessage(t, result.Message, "requires explicit confirmation")
}
