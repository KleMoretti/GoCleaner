package cleaner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gocleaner/internal/model"
)

func TestShredFileOverwritesAndDeletesRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("secret", 128)), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := ShredFile(model.ShredRequest{Path: path, Passes: 1}, true)
	if err != nil {
		t.Fatalf("ShredFile returned error: %v", err)
	}
	if result.ShreddedFiles != 1 {
		t.Fatalf("ShreddedFiles = %d, want 1", result.ShreddedFiles)
	}
	if result.FreedSize == 0 {
		t.Fatalf("FreedSize = %d, want positive", result.FreedSize)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("original file should be gone, stat err = %v", statErr)
	}
}

func TestShredFileRequiresConfirmation(t *testing.T) {
	path := writeTestFile(t, t.TempDir(), "secret.txt", "secret")

	result, err := ShredFile(model.ShredRequest{Path: path, Passes: 1}, false)
	if err == nil {
		t.Fatal("ShredFile error = nil, want confirmation error")
	}
	if result == nil || result.ShreddedFiles != 0 {
		t.Fatalf("result = %+v, want no shredded files", result)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("file should remain without confirmation: %v", statErr)
	}
}

func TestShredFileRejectsInvalidPassesAndDirectories(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "secret.txt", "secret")

	result, err := ShredFile(model.ShredRequest{Path: path, Passes: 2}, true)
	if err != nil {
		t.Fatalf("invalid passes should be reported in result, not returned as fatal error: %v", err)
	}
	if len(result.FailedFiles) != 1 || !strings.Contains(result.FailedReasons[path], "粉碎次数") {
		t.Fatalf("invalid passes result = %+v", result)
	}

	result, err = ShredFile(model.ShredRequest{Path: dir, Passes: 1}, true)
	if err != nil {
		t.Fatalf("directory failure should be reported in result, not returned as fatal error: %v", err)
	}
	if len(result.FailedFiles) != 1 || !strings.Contains(result.FailedReasons[dir], "目录") {
		t.Fatalf("directory failure result = %+v", result)
	}
}

func TestShredFileRejectsMissingFilesAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.txt")

	result, err := ShredFile(model.ShredRequest{Path: missing, Passes: 1}, true)
	if err != nil {
		t.Fatalf("missing file should be reported in result, not returned as fatal error: %v", err)
	}
	if len(result.FailedFiles) != 1 || !strings.Contains(result.FailedReasons[missing], "路径不存在") {
		t.Fatalf("missing file result = %+v", result)
	}

	target := writeTestFile(t, dir, "target.txt", "secret")
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}

	result, err = ShredFile(model.ShredRequest{Path: link, Passes: 1}, true)
	if err != nil {
		t.Fatalf("symlink should be reported in result, not returned as fatal error: %v", err)
	}
	if len(result.FailedFiles) != 1 || !strings.Contains(result.FailedReasons[link], "符号链接") {
		t.Fatalf("symlink failure result = %+v", result)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("symlink target should remain: %v", statErr)
	}
}
