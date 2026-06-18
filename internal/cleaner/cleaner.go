// Package cleaner executes file cleanup operations from scanner results.
package cleaner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gocleaner/internal/model"
)

var ErrHighRiskConfirmationRequired = errors.New("high-risk confirmation required")

type Options struct {
	HighRiskConfirmed bool
}

func Clean(items []model.ScanItem, options Options) (*model.CleanResult, error) {
	result := &model.CleanResult{
		FailedFiles:   make([]string, 0),
		FailedReasons: make(map[string]string),
	}

	selected := selectedItems(items)
	if containsHighRisk(selected) && !options.HighRiskConfirmed {
		result.Message = "High-risk items require explicit confirmation before cleaning."
		return result, ErrHighRiskConfirmationRequired
	}

	for _, item := range selected {
		cleanItem(item, result)
	}

	result.Message = fmt.Sprintf("Deleted %d file(s), freed %d byte(s), failed %d item(s).",
		result.DeletedFiles,
		result.FreedSize,
		len(result.FailedFiles),
	)
	return result, nil
}

func selectedItems(items []model.ScanItem) []model.ScanItem {
	selected := make([]model.ScanItem, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if !item.Selected {
			continue
		}
		key := cleanItemDedupKey(item)
		if key != "" && seen[key] {
			continue
		}
		if key != "" {
			seen[key] = true
		}
		selected = append(selected, item)
	}
	return selected
}

func cleanItemDedupKey(item model.ScanItem) string {
	if strings.TrimSpace(item.Path) == "" {
		return ""
	}
	return item.Type + "\x00" + strings.ToLower(filepath.Clean(item.Path))
}

func containsHighRisk(items []model.ScanItem) bool {
	for _, item := range items {
		if item.Risk == model.RiskHigh {
			return true
		}
	}
	return false
}

func cleanItem(item model.ScanItem, result *model.CleanResult) {
	if item.Type != model.TypeFile {
		recordFailure(result, item.Path, fmt.Sprintf("unsupported item type: %s", item.Type))
		return
	}
	if strings.TrimSpace(item.Path) == "" {
		recordFailure(result, item.Path, "empty path")
		return
	}

	info, err := os.Lstat(item.Path)
	if err != nil {
		if os.IsNotExist(err) {
			recordFailure(result, item.Path, "not found")
			return
		}
		recordFailure(result, item.Path, classifyAccessError(err))
		return
	}

	if info.Mode()&os.ModeSymlink != 0 {
		recordFailure(result, item.Path, "symbolic link skipped")
		return
	}
	if !info.Mode().IsRegular() {
		recordFailure(result, item.Path, "not a regular file")
		return
	}

	size := info.Size()
	if err := os.Remove(item.Path); err != nil {
		recordFailure(result, item.Path, classifyAccessError(err))
		return
	}

	result.DeletedFiles++
	result.FreedSize += size
}

func recordFailure(result *model.CleanResult, path, reason string) {
	result.FailedFiles = append(result.FailedFiles, path)
	result.FailedReasons[path] = reason
}

func classifyAccessError(err error) string {
	message := err.Error()
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "being used by another process"),
		strings.Contains(lower, "process cannot access"),
		strings.Contains(lower, "sharing violation"),
		strings.Contains(lower, "file is locked"):
		return "file locked or in use: " + message
	case strings.Contains(lower, "access is denied"),
		strings.Contains(lower, "permission denied"):
		return "permission denied: " + message
	default:
		return "delete failed: " + message
	}
}
