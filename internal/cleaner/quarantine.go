package cleaner

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gocleaner/internal/model"
	"gocleaner/internal/paths"
)

// QuarantineStore manages plugin directories moved out of browser extension roots.
type QuarantineStore struct {
	root string
}

var renamePath = os.Rename

const maxQuarantineRecordLineBytes = 16 * 1024 * 1024

// NewQuarantineStore creates a quarantine store rooted at the given directory.
func NewQuarantineStore(root string) *QuarantineStore {
	return &QuarantineStore{root: root}
}

// DefaultQuarantineRoot is the default app-data plugin quarantine path.
func DefaultQuarantineRoot() string {
	return paths.PluginQuarantineDir()
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

	result.Message = fmt.Sprintf("已隔离 %d 个插件，失败 %d 项。",
		result.MovedItems,
		len(result.FailedItems),
	)
	return result, nil
}

// ListRecords returns the current quarantine records, newest first.
func (s *QuarantineStore) ListRecords() ([]model.QuarantineRecord, error) {
	records, err := s.readRecords()
	if err != nil {
		return nil, fmt.Errorf("读取隔离记录失败: %w", err)
	}
	sortRecordsNewestFirst(records)
	return records, nil
}

// RestorePlugin moves one quarantined plugin directory back to its original path.
func (s *QuarantineStore) RestorePlugin(recordID string) (*model.QuarantineResult, error) {
	result := newQuarantineResult()
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		recordQuarantineFailure(result, recordID, "隔离记录 ID 为空")
		result.Message = "恢复失败：隔离记录 ID 为空。"
		return result, nil
	}

	records, err := s.readRecords()
	if err != nil {
		return nil, fmt.Errorf("读取隔离记录失败: %w", err)
	}
	index := -1
	for i := range records {
		if records[i].RecordID == recordID {
			index = i
			break
		}
	}
	if index < 0 {
		recordQuarantineFailure(result, recordID, "未找到隔离记录")
		result.Message = "恢复失败：未找到隔离记录。"
		return result, nil
	}
	record := records[index]
	if strings.TrimSpace(record.RestoredAt) != "" {
		recordQuarantineFailure(result, record.OriginalPath, "隔离记录已恢复")
		result.Message = "恢复失败：隔离记录已恢复。"
		return result, nil
	}
	if _, err := os.Stat(record.OriginalPath); err == nil {
		recordQuarantineFailure(result, record.OriginalPath, "原始路径已存在")
		result.Message = "恢复失败：原始路径已存在。"
		return result, nil
	}
	if _, err := os.Stat(record.QuarantinePath); err != nil {
		recordQuarantineFailure(result, record.QuarantinePath, "隔离内容缺失："+err.Error())
		result.Message = "恢复失败：隔离内容缺失。"
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(record.OriginalPath), 0o755); err != nil {
		recordQuarantineFailure(result, record.OriginalPath, "创建原始父目录失败："+err.Error())
		result.Message = "恢复失败：无法创建原始父目录。"
		return result, nil
	}
	if err := movePath(record.QuarantinePath, record.OriginalPath); err != nil {
		recordQuarantineFailure(result, record.OriginalPath, "恢复移动失败："+err.Error())
		result.Message = "恢复失败：移动失败。"
		return result, nil
	}

	records[index].RestoredAt = time.Now().Format(time.RFC3339)
	if err := s.writeRecords(records); err != nil {
		if rollbackErr := movePath(record.OriginalPath, record.QuarantinePath); rollbackErr != nil {
			return result, fmt.Errorf("写入恢复记录失败: %w；回滚失败：%v", err, rollbackErr)
		}
		return result, fmt.Errorf("写入恢复记录失败: %w", err)
	}

	result.RestoredItems = 1
	result.Message = "已从隔离区恢复 1 个插件。"
	return result, nil
}

func (s *QuarantineStore) quarantinePlugin(item model.ScanItem) error {
	if item.Type != model.TypePlugin {
		return fmt.Errorf("不支持的隔离项目类型：%s，仅支持插件项", item.Type)
	}
	if strings.TrimSpace(item.Path) == "" {
		return fmt.Errorf("路径为空")
	}
	info, err := os.Lstat(item.Path)
	if err != nil {
		return fmt.Errorf("插件路径不可访问: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("已跳过符号链接")
	}
	if !info.IsDir() {
		return fmt.Errorf("插件项目不是目录")
	}
	if !isBrowserExtensionRoot(item.Path) {
		return fmt.Errorf("插件隔离仅支持浏览器 Extensions 目录")
	}
	if !hasPluginManifest(item.Path) {
		return fmt.Errorf("插件目录下未找到 manifest.json")
	}

	recordID := quarantineRecordID(item.Path)
	recordDir := filepath.Join(s.root, recordID)
	contentPath := filepath.Join(recordDir, "content")
	if err := os.MkdirAll(recordDir, 0o755); err != nil {
		return fmt.Errorf("创建隔离目录失败: %w", err)
	}
	if _, err := os.Stat(contentPath); err == nil {
		return fmt.Errorf("隔离目标已存在")
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
		return fmt.Errorf("写入隔离记录失败: %w", err)
	}
	if err := movePath(item.Path, contentPath); err != nil {
		if cleanupErr := s.removeRecord(recordID); cleanupErr != nil {
			return fmt.Errorf("移动到隔离区失败: %w；清理记录失败：%v", err, cleanupErr)
		}
		_ = os.Remove(recordDir)
		return fmt.Errorf("移动到隔离区失败: %w", err)
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
	scanner.Buffer(make([]byte, 64*1024), maxQuarantineRecordLineBytes)
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

func movePath(src, dst string) error {
	if err := renamePath(src, dst); err == nil {
		return nil
	}
	if err := copyDir(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	if err := os.RemoveAll(src); err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	return nil
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("已跳过符号链接")
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("源路径不是目录")
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("已跳过符号链接：%s", path)
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
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
