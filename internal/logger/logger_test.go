package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gocleaner/internal/model"
)

func TestAppendCreatesDirectoryAndWritesJSONLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "operation.jsonl")
	store := New(path)
	entry := model.OperationLog{
		Timestamp:    "2026-06-11T10:00:00+08:00",
		Operation:    model.OpClean,
		DeletedFiles: 2,
		FreedSize:    128,
		FailedPaths:  []string{"C:\\locked.tmp"},
	}

	if err := store.Append(entry); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile operation log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1", len(lines))
	}

	var decoded model.OperationLog
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("log line is not valid JSON: %v", err)
	}
	if decoded.Operation != model.OpClean || decoded.DeletedFiles != 2 || decoded.FreedSize != 128 {
		t.Fatalf("decoded log = %+v, want clean/deleted=2/freed=128", decoded)
	}
}

func TestReadRecentReturnsNewestEntriesFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operation.jsonl")
	store := New(path)
	entries := []model.OperationLog{
		{Timestamp: "2026-06-11T10:00:00+08:00", Operation: model.OpScan, ScannedFiles: 10},
		{Timestamp: "2026-06-11T10:01:00+08:00", Operation: model.OpClean, DeletedFiles: 2},
		{Timestamp: "2026-06-11T10:02:00+08:00", Operation: model.OpShred, DeletedFiles: 1},
	}

	for _, entry := range entries {
		if err := store.Append(entry); err != nil {
			t.Fatalf("Append returned error: %v", err)
		}
	}

	got, err := store.ReadRecent(2)
	if err != nil {
		t.Fatalf("ReadRecent returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("ReadRecent len = %d, want 2", len(got))
	}
	if got[0].Operation != model.OpShred || got[1].Operation != model.OpClean {
		t.Fatalf("ReadRecent operations = %q, %q; want shred, clean", got[0].Operation, got[1].Operation)
	}
}

func TestReadRecentMissingFileReturnsEmpty(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "operation.jsonl"))

	got, err := store.ReadRecent(10)
	if err != nil {
		t.Fatalf("ReadRecent returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReadRecent len = %d, want 0", len(got))
	}
}

func TestReadRecentSkipsMalformedJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operation.jsonl")
	goodOld := `{"timestamp":"2026-06-11T10:00:00+08:00","operation":"scan","scanned_files":1}`
	goodNew := `{"timestamp":"2026-06-11T10:01:00+08:00","operation":"clean","deleted_files":1}`
	content := strings.Join([]string{goodOld, "{broken json", goodNew}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile log: %v", err)
	}

	got, err := New(path).ReadRecent(10)
	if err != nil {
		t.Fatalf("ReadRecent returned error for one bad line: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ReadRecent len = %d, want 2 valid entries", len(got))
	}
	if got[0].Operation != model.OpClean || got[1].Operation != model.OpScan {
		t.Fatalf("operations = %q, %q; want clean, scan", got[0].Operation, got[1].Operation)
	}
}
