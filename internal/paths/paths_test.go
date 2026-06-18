package paths

import (
	"path/filepath"
	"testing"
)

func TestDataDirUsesEnvironmentOverride(t *testing.T) {
	root := filepath.Join(t.TempDir(), "gocleaner-data")
	t.Setenv(DataDirEnv, root)

	if got := DataDir(); got != filepath.Clean(root) {
		t.Fatalf("DataDir = %q, want %q", got, filepath.Clean(root))
	}
	if got := OperationLogPath(); got != filepath.Join(filepath.Clean(root), "operation.jsonl") {
		t.Fatalf("OperationLogPath = %q", got)
	}
	if got := RegistryBackupDir(); got != filepath.Join(filepath.Clean(root), "registry_backup") {
		t.Fatalf("RegistryBackupDir = %q", got)
	}
	if got := PluginQuarantineDir(); got != filepath.Join(filepath.Clean(root), "quarantine", "plugins") {
		t.Fatalf("PluginQuarantineDir = %q", got)
	}
}
