//go:build windows

package cleaner

import (
	"os"
	"strings"
	"testing"

	"gocleaner/internal/model"

	"golang.org/x/sys/windows"
)

func TestCleanSkipsLockedFileAndRecordsReason(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "locked.tmp", "hello")

	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}

	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("CreateFile lock handle: %v", err)
	}
	defer windows.CloseHandle(handle)

	result, cleanErr := Clean([]model.ScanItem{testItem(path, true, model.RiskLow)}, Options{})
	if cleanErr != nil {
		t.Fatalf("Clean returned error: %v", cleanErr)
	}

	if result.DeletedFiles != 0 {
		t.Fatalf("DeletedFiles = %d, want 0", result.DeletedFiles)
	}
	if len(result.FailedFiles) != 1 || result.FailedFiles[0] != path {
		t.Fatalf("FailedFiles = %v, want [%s]", result.FailedFiles, path)
	}
	reason := strings.ToLower(result.FailedReasons[path])
	if !strings.Contains(reason, "占用") {
		t.Fatalf("failure reason = %q, want 文件被占用", result.FailedReasons[path])
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("locked file should remain: %v", statErr)
	}
}
