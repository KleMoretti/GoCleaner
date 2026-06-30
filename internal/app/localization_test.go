package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gocleaner/internal/model"
)

func assertChineseAppMessage(t *testing.T, value string, banned ...string) {
	t.Helper()
	if strings.TrimSpace(value) == "" {
		t.Fatal("app message is empty")
	}
	for _, phrase := range banned {
		if strings.Contains(strings.ToLower(value), strings.ToLower(phrase)) {
			t.Fatalf("app message %q should not contain English phrase %q", value, phrase)
		}
	}
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			return
		}
	}
	t.Fatalf("app message %q should contain Chinese text", value)
}

func TestAppRuntimeMessagesAreChinese(t *testing.T) {
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
	assertChineseAppMessage(t, result.Message, "Deleted", "freed", "failed")
	assertChineseAppMessage(t, result.FailedReasons[path], "latest scan", "run scan again")
	assertChineseAppMessage(t, failurePath(model.ScanItem{}), "unknown item")
}

func TestLogWarningIsChinese(t *testing.T) {
	assertChineseAppMessage(t, logWarning(os.ErrPermission), "operation completed", "operation log was not recorded")
}
