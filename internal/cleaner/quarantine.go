package cleaner

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gocleaner/internal/model"
)

// QuarantineStore manages plugin directories moved out of browser extension roots.
type QuarantineStore struct {
	root string
}

// NewQuarantineStore creates a quarantine store rooted at the given directory.
func NewQuarantineStore(root string) *QuarantineStore {
	return &QuarantineStore{root: root}
}

// DefaultQuarantineRoot is the default project-local plugin quarantine path.
func DefaultQuarantineRoot() string {
	return filepath.Join("data", "quarantine", "plugins")
}

// QuarantinePlugins moves selected plugin directories into the quarantine store.
func (s *QuarantineStore) QuarantinePlugins(items []model.ScanItem) (*model.QuarantineResult, error) {
	result := newQuarantineResult()

	for _, item := range items {
		if !item.Selected {
			continue
		}
		if err := s.quarantinePlugin(item); err != nil {
			recordQuarantineFailure(result, item.Path, err.Error())
			continue
		}
		result.MovedItems++
	}

	result.Message = fmt.Sprintf("Moved %d plugin(s) to quarantine, failed %d item(s).",
		result.MovedItems,
		len(result.FailedItems),
	)
	return result, nil
}

// ListRecords returns the current quarantine records, newest first.
func (s *QuarantineStore) ListRecords() ([]model.QuarantineRecord, error) {
	records, err := s.readRecords()
	if err != nil {
		return nil, err
	}
	sortRecordsNewestFirst(records)
	return records, nil
}

// RestorePlugin moves one quarantined plugin directory back to its original path.
func (s *QuarantineStore) RestorePlugin(recordID string) (*model.QuarantineResult, error) {
	result := newQuarantineResult()
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		recordQuarantineFailure(result, recordID, "empty quarantine record id")
		result.Message = "Restore failed: empty quarantine record id."
		return result, nil
	}

	records, err := s.readRecords()
	if err != nil {
		return nil, err
	}
	index := -1
	for i := range records {
		if records[i].RecordID == recordID {
			index = i
			break
		}
	}
	if index < 0 {
		recordQuarantineFailure(result, recordID, "quarantine record not found")
		result.Message = "Restore failed: quarantine record not found."
		return result, nil
	}
	record := records[index]
	if strings.TrimSpace(record.RestoredAt) != "" {
		recordQuarantineFailure(result, record.OriginalPath, "quarantine record already restored")
		result.Message = "Restore failed: quarantine record already restored."
		return result, nil
	}
	if _, err := os.Stat(record.OriginalPath); err == nil {
		recordQuarantineFailure(result, record.OriginalPath, "original path already exists")
		result.Message = "Restore failed: original path already exists."
		return result, nil
	}
	if _, err := os.Stat(record.QuarantinePath); err != nil {
		recordQuarantineFailure(result, record.QuarantinePath, "quarantined content missing: "+err.Error())
		result.Message = "Restore failed: quarantined content missing."
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(record.OriginalPath), 0o755); err != nil {
		recordQuarantineFailure(result, record.OriginalPath, "create original parent failed: "+err.Error())
		result.Message = "Restore failed: cannot create original parent."
		return result, nil
	}
	if err := os.Rename(record.QuarantinePath, record.OriginalPath); err != nil {
		recordQuarantineFailure(result, record.OriginalPath, "restore move failed: "+err.Error())
		result.Message = "Restore failed: move failed."
		return result, nil
	}

	records[index].RestoredAt = time.Now().Format(time.RFC3339)
	if err := s.writeRecords(records); err != nil {
		if rollbackErr := os.Rename(record.OriginalPath, record.QuarantinePath); rollbackErr != nil {
			return result, fmt.Errorf("write restore record failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return result, err
	}

	result.RestoredItems = 1
	result.Message = "Restored 1 plugin from quarantine."
	return result, nil
}

func (s *QuarantineStore) quarantinePlugin(item model.ScanItem) error {
	if item.Type != model.TypePlugin {
		return fmt.Errorf("unsupported item type for plugin quarantine: %s", item.Type)
	}
	if strings.TrimSpace(item.Path) == "" {
		return fmt.Errorf("empty path")
	}
	info, err := os.Lstat(item.Path)
	if err != nil {
		return fmt.Errorf("plugin path not accessible: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic link skipped")
	}
	if !info.IsDir() {
		return fmt.Errorf("plugin item is not a directory")
	}
	if !isBrowserExtensionRoot(item.Path) {
		return fmt.Errorf("plugin quarantine only supports browser Extensions directories")
	}
	if !hasPluginManifest(item.Path) {
		return fmt.Errorf("plugin manifest not found under extension directory")
	}

	recordID := quarantineRecordID(item.Path)
	recordDir := filepath.Join(s.root, recordID)
	contentPath := filepath.Join(recordDir, "content")
	if err := os.MkdirAll(recordDir, 0o755); err != nil {
		return fmt.Errorf("create quarantine directory failed: %w", err)
	}
	if _, err := os.Stat(contentPath); err == nil {
		return fmt.Errorf("quarantine target already exists")
	}

	size := quarantineDirectorySize(item.Path)
	record := model.QuarantineRecord{
		RecordID:       recordID,
		OriginalPath:   item.Path,
		QuarantinePath: contentPath,
		Name:           item.Name,
		ItemType:       item.Type,
		CreatedAt:      time.Now().Format(time.RFC3339),
		Size:           size,
	}
	if item.Plugin != nil {
		record.Browser = item.Plugin.Browser
	}
	if err := s.appendRecord(record); err != nil {
		return fmt.Errorf("write quarantine record failed: %w", err)
	}
	if err := os.Rename(item.Path, contentPath); err != nil {
		if cleanupErr := s.removeRecord(recordID); cleanupErr != nil {
			return fmt.Errorf("move to quarantine failed: %w; cleanup record failed: %v", err, cleanupErr)
		}
		_ = os.Remove(recordDir)
		return fmt.Errorf("move to quarantine failed: %w", err)
	}

	return nil
}

func isBrowserExtensionRoot(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	return strings.Contains(normalized, "/extensions/") && !strings.HasSuffix(normalized, "/extensions")
}

func hasPluginManifest(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, entry.Name(), "manifest.json")); err == nil {
			return true
		}
	}
	return false
}

func newQuarantineResult() *model.QuarantineResult {
	return &model.QuarantineResult{
		FailedItems:   make([]string, 0),
		FailedReasons: make(map[string]string),
	}
}

func recordQuarantineFailure(result *model.QuarantineResult, path, reason string) {
	result.FailedItems = append(result.FailedItems, path)
	result.FailedReasons[path] = reason
}

func quarantineRecordID(path string) string {
	hash := sha1.Sum([]byte(fmt.Sprintf("%s-%d", path, time.Now().UnixNano())))
	return hex.EncodeToString(hash[:])[:16]
}

func (s *QuarantineStore) indexPath() string {
	return filepath.Join(s.root, "index.jsonl")
}

func (s *QuarantineStore) appendRecord(record model.QuarantineRecord) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.indexPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *QuarantineStore) removeRecord(recordID string) error {
	records, err := s.readRecords()
	if err != nil {
		return err
	}
	filtered := records[:0]
	for _, record := range records {
		if record.RecordID != recordID {
			filtered = append(filtered, record)
		}
	}
	return s.writeRecords(filtered)
}

func (s *QuarantineStore) readRecords() ([]model.QuarantineRecord, error) {
	f, err := os.Open(s.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []model.QuarantineRecord{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []model.QuarantineRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record model.QuarantineRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *QuarantineStore) writeRecords(records []model.QuarantineRecord) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	indexPath := s.indexPath()
	tmpPath := fmt.Sprintf("%s.tmp.%d", indexPath, time.Now().UnixNano())
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	for _, record := range records {
		encoded, err := json.Marshal(record)
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return err
		}
		if _, err := f.Write(append(encoded, '\n')); err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := replaceFile(tmpPath, indexPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func sortRecordsNewestFirst(records []model.QuarantineRecord) {
	for i := 0; i < len(records); i++ {
		for j := i + 1; j < len(records); j++ {
			if records[j].CreatedAt > records[i].CreatedAt {
				records[i], records[j] = records[j], records[i]
			}
		}
	}
}

func quarantineDirectorySize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
