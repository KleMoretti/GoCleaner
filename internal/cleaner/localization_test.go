package cleaner

import (
	"strings"
	"testing"

	"gocleaner/internal/model"
)

func assertChineseRuntimeMessage(t *testing.T, value string, banned ...string) {
	t.Helper()
	if strings.TrimSpace(value) == "" {
		t.Fatal("runtime message is empty")
	}
	for _, phrase := range banned {
		if strings.Contains(strings.ToLower(value), strings.ToLower(phrase)) {
			t.Fatalf("runtime message %q should not contain English phrase %q", value, phrase)
		}
	}
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			return
		}
	}
	t.Fatalf("runtime message %q should contain Chinese text", value)
}

func TestCleanRuntimeErrorsAreChinese(t *testing.T) {
	path := writeTestFile(t, t.TempDir(), "high-risk.tmp", "hello")
	result, err := Clean([]model.ScanItem{testItem(path, true, model.RiskHigh)}, Options{})
	if err == nil {
		t.Fatal("Clean error = nil, want confirmation error")
	}
	assertChineseRuntimeMessage(t, err.Error(), "high-risk", "confirmation required")
	assertChineseRuntimeMessage(t, result.Message, "High-risk", "requires", "cleaning")

	missing := path + ".missing"
	result, err = Clean([]model.ScanItem{testItem(missing, true, model.RiskLow)}, Options{})
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}
	assertChineseRuntimeMessage(t, result.Message, "Deleted", "freed", "failed")
	assertChineseRuntimeMessage(t, result.FailedReasons[missing], "not found")
}

func TestQuarantineRuntimeErrorsAreChinese(t *testing.T) {
	filePath := writeTestFile(t, t.TempDir(), "cache.tmp", "hello")
	store := NewQuarantineStore(t.TempDir())

	result, err := store.QuarantinePlugins([]model.ScanItem{testItem(filePath, true, model.RiskLow)})
	if err != nil {
		t.Fatalf("QuarantinePlugins returned error: %v", err)
	}
	assertChineseRuntimeMessage(t, result.Message, "Moved", "quarantine", "failed")
	assertChineseRuntimeMessage(t, result.FailedReasons[filePath], "unsupported", "plugin quarantine")

	restore, err := store.RestorePlugin("")
	if err != nil {
		t.Fatalf("RestorePlugin returned error: %v", err)
	}
	assertChineseRuntimeMessage(t, restore.Message, "Restore failed", "empty quarantine record id")
	if len(restore.FailedItems) != 1 {
		t.Fatalf("restore failures = %+v, want one", restore.FailedItems)
	}
	assertChineseRuntimeMessage(t, restore.FailedReasons[restore.FailedItems[0]], "empty quarantine record id")
}

func TestShredRuntimeErrorsAreChinese(t *testing.T) {
	path := writeTestFile(t, t.TempDir(), "secret.txt", "secret")

	result, err := ShredFile(model.ShredRequest{Path: path, Passes: 1}, false)
	if err == nil {
		t.Fatal("ShredFile error = nil, want confirmation error")
	}
	assertChineseRuntimeMessage(t, err.Error(), "high-risk", "confirmation required")
	assertChineseRuntimeMessage(t, result.Message, "requires explicit confirmation")

	result, err = ShredFile(model.ShredRequest{Path: path, Passes: 2}, true)
	if err != nil {
		t.Fatalf("ShredFile returned error: %v", err)
	}
	assertChineseRuntimeMessage(t, result.Message, "Shredded", "failed")
	assertChineseRuntimeMessage(t, result.FailedReasons[path], "invalid", "passes")
}
