// Package paths centralizes writable runtime paths for GoCleaner.
package paths

import (
	"os"
	"path/filepath"
	"strings"
)

const DataDirEnv = "GOCLEANER_DATA_DIR"

func DataDir() string {
	if override := strings.TrimSpace(os.Getenv(DataDirEnv)); override != "" {
		return filepath.Clean(override)
	}
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "GoCleaner")
	}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return filepath.Join(filepath.Dir(exe), "data")
	}
	return filepath.Join("data")
}

func OperationLogPath() string {
	return filepath.Join(DataDir(), "operation.jsonl")
}

func RegistryBackupDir() string {
	return filepath.Join(DataDir(), "registry_backup")
}

func PluginQuarantineDir() string {
	return filepath.Join(DataDir(), "quarantine", "plugins")
}
