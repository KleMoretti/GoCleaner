// Package registry implements the limited, high-risk registry workflow for GoCleaner.
package registry

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"gocleaner/internal/model"
	"gocleaner/internal/windows"
)

const RunKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

var ErrRegistryConfirmationRequired = errors.New("registry delete confirmation required")

// ScanInvalidStartup scans only HKCU Run values and returns invalid startup targets.
func ScanInvalidStartup() (*model.ScanResult, error) {
	start := time.Now()
	values, err := windows.ReadHKCUValues(RunKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read HKCU startup values: %w", err)
	}

	result := &model.ScanResult{
		Items:  BuildInvalidStartupItems(values, fileExists),
		Errors: make([]model.ScanError, 0),
	}
	result.TotalFiles = len(result.Items)
	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

// BuildInvalidStartupItems converts registry startup values into high-risk scan items.
func BuildInvalidStartupItems(values []windows.RegistryValue, exists func(string) bool) []model.ScanItem {
	items := make([]model.ScanItem, 0)
	for _, value := range values {
		if value.Type != windows.RegistryString && value.Type != windows.RegistryExpandString {
			continue
		}

		target, ok := ParseStartupCommand(value.Data)
		if !ok {
			continue
		}
		if exists(target) {
			continue
		}

		path := `HKCU\` + RunKeyPath + `\` + value.Name
		items = append(items, model.ScanItem{
			ID:       registryItemID(path),
			Path:     path,
			Name:     value.Name,
			Type:     model.TypeRegistry,
			Category: model.CategoryRegistry,
			Size:     0,
			Risk:     model.RiskHigh,
			Source:   "HKCU 启动项",
			Selected: false,
			Registry: &model.RegistryInfo{
				Hive:         "HKCU",
				KeyPath:      RunKeyPath,
				ValueName:    value.Name,
				ValueType:    value.Type,
				RawData:      value.Data,
				ExpandedPath: target,
				TargetPath:   target,
			},
		})
	}
	return items
}

// ParseStartupCommand extracts a local absolute executable path from a startup command.
func ParseStartupCommand(raw string) (string, bool) {
	command := strings.TrimSpace(raw)
	if command == "" {
		return "", false
	}

	var candidate string
	if strings.HasPrefix(command, `"`) {
		rest := strings.TrimPrefix(command, `"`)
		end := strings.Index(rest, `"`)
		if end < 0 {
			return "", false
		}
		candidate = rest[:end]
	} else {
		fields := strings.Fields(command)
		if len(fields) == 0 {
			return "", false
		}
		candidate = fields[0]
		if len(fields) > 1 && looksLikeAmbiguousUnquotedPath(candidate) {
			return "", false
		}
	}

	candidate = strings.TrimSpace(candidate)
	candidate = strings.Trim(candidate, `"`)
	if candidate == "" {
		return "", false
	}

	expanded := windows.ExpandPath(candidate)
	if strings.Contains(expanded, "%") {
		return "", false
	}
	if !filepath.IsAbs(expanded) {
		return "", false
	}
	return expanded, true
}

func looksLikeAmbiguousUnquotedPath(candidate string) bool {
	expanded := windows.ExpandPath(strings.Trim(candidate, `"`))
	if strings.Contains(expanded, "%") || !filepath.IsAbs(expanded) {
		return false
	}
	return filepath.Ext(expanded) == ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func registryItemID(path string) string {
	hash := md5.Sum([]byte(path))
	return fmt.Sprintf("%s_%x", model.CategoryRegistry, hash[:8])
}

// DeleteRegistryItems backs up selected registry values and then deletes those values.
func DeleteRegistryItems(items []model.ScanItem, confirmed bool) (*model.RegistryActionResult, error) {
	return DeleteRegistryItemsWithBackupDir(items, confirmed, DefaultBackupDir())
}

// DeleteRegistryItemsWithBackupDir is the testable implementation behind DeleteRegistryItems.
func DeleteRegistryItemsWithBackupDir(items []model.ScanItem, confirmed bool, backupDir string) (*model.RegistryActionResult, error) {
	result := newRegistryActionResult()
	selected := selectedRegistryItems(items)
	if len(selected) == 0 {
		result.Message = "No registry values selected."
		return result, nil
	}
	if !confirmed {
		result.Message = "Registry deletion requires explicit confirmation."
		return result, ErrRegistryConfirmationRequired
	}

	supported := make([]model.ScanItem, 0, len(selected))
	for _, item := range selected {
		if item.Type != model.TypeRegistry || item.Registry == nil || item.Registry.Hive != "HKCU" || item.Registry.KeyPath != RunKeyPath {
			recordRegistryFailure(result, item.Path, "unsupported registry target")
			continue
		}
		supported = append(supported, item)
	}
	if len(supported) == 0 {
		result.Message = "No supported HKCU Run registry values selected."
		return result, nil
	}

	backupPath := registryBackupPath(backupDir, time.Now())
	if err := WriteRegistryBackup(backupPath, supported); err != nil {
		for _, item := range supported {
			recordRegistryFailure(result, item.Path, "registry backup failed: "+err.Error())
		}
		result.Message = "Registry backup failed; no values were deleted."
		return result, nil
	}
	result.BackupPath = backupPath

	for _, item := range supported {
		if err := windows.DeleteHKCUValue(item.Registry.KeyPath, item.Registry.ValueName); err != nil {
			recordRegistryFailure(result, item.Path, "delete registry value failed: "+err.Error())
			continue
		}
		result.DeletedValues++
	}

	result.Message = fmt.Sprintf("Deleted %d registry value(s), backup: %s, failed %d item(s).",
		result.DeletedValues,
		result.BackupPath,
		len(result.FailedItems),
	)
	return result, nil
}

func registryBackupPath(backupDir string, now time.Time) string {
	name := fmt.Sprintf("registry_backup_%s_%09d.reg", now.Format("20060102_150405"), now.Nanosecond())
	return filepath.Join(backupDir, name)
}

func selectedRegistryItems(items []model.ScanItem) []model.ScanItem {
	selected := make([]model.ScanItem, 0, len(items))
	for _, item := range items {
		if item.Selected {
			selected = append(selected, item)
		}
	}
	return selected
}

func newRegistryActionResult() *model.RegistryActionResult {
	return &model.RegistryActionResult{
		FailedItems:   make([]string, 0),
		FailedReasons: make(map[string]string),
	}
}

func recordRegistryFailure(result *model.RegistryActionResult, path, reason string) {
	result.FailedItems = append(result.FailedItems, path)
	result.FailedReasons[path] = reason
}

// DefaultBackupDir returns the project-local registry backup directory.
func DefaultBackupDir() string {
	return filepath.Join("data", "registry_backup")
}

// WriteRegistryBackup writes a .reg backup for selected registry values.
func WriteRegistryBackup(path string, items []model.ScanItem) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty backup path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("Windows Registry Editor Version 5.00\r\n\r\n")
	b.WriteString(`[HKEY_CURRENT_USER\` + RunKeyPath + "]\r\n")
	for _, item := range items {
		if item.Registry == nil {
			continue
		}
		name := escapeRegString(item.Registry.ValueName)
		switch item.Registry.ValueType {
		case windows.RegistryExpandString:
			b.WriteString(fmt.Sprintf(`"%s"=hex(2):%s`, name, encodeExpandString(item.Registry.RawData)))
		default:
			b.WriteString(fmt.Sprintf(`"%s"="%s"`, name, escapeRegString(item.Registry.RawData)))
		}
		b.WriteString("\r\n")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write([]byte(b.String())); err != nil {
		return err
	}
	return nil
}

func escapeRegString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func encodeExpandString(value string) string {
	encoded := utf16.Encode([]rune(value + "\x00"))
	bytes := make([]byte, 0, len(encoded)*2)
	for _, unit := range encoded {
		bytes = append(bytes, byte(unit), byte(unit>>8))
	}
	parts := make([]string, 0, len(bytes))
	for _, b := range bytes {
		dst := make([]byte, 2)
		hex.Encode(dst, []byte{b})
		parts = append(parts, string(dst))
	}
	return strings.Join(parts, ",")
}
