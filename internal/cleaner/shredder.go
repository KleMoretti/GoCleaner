package cleaner

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gocleaner/internal/model"
)

var allowedShredPasses = map[int]bool{1: true, 3: true, 7: true}

// ShredFile overwrites, flushes, renames, and deletes one explicit user-selected file.
func ShredFile(request model.ShredRequest, confirmed bool) (*model.ShredResult, error) {
	result := newShredResult()
	if !confirmed {
		result.Message = "文件粉碎需要明确确认。"
		return result, ErrHighRiskConfirmationRequired
	}

	path := strings.TrimSpace(request.Path)
	if path == "" {
		recordShredFailure(result, request.Path, "路径为空")
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}
	if !allowedShredPasses[request.Passes] {
		recordShredFailure(result, path, "粉碎次数无效；允许值为 1、3 或 7")
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}

	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			recordShredFailure(result, path, "路径不存在")
		} else {
			recordShredFailure(result, path, classifyAccessError(err))
		}
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		recordShredFailure(result, path, "已跳过符号链接")
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}
	if info.IsDir() {
		recordShredFailure(result, path, "目录不能粉碎")
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}
	if !info.Mode().IsRegular() {
		recordShredFailure(result, path, "不是普通文件")
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}

	size := info.Size()
	if err := overwriteFile(path, size, request.Passes); err != nil {
		recordShredFailure(result, path, err.Error())
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}

	renamedPath, err := randomSiblingPath(path)
	if err != nil {
		recordShredFailure(result, path, "生成随机重命名路径失败："+err.Error())
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}
	if err := os.Rename(path, renamedPath); err != nil {
		recordShredFailure(result, path, "随机重命名失败："+err.Error())
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}
	if err := os.Remove(renamedPath); err != nil {
		recordShredFailure(result, renamedPath, "覆写后删除失败："+err.Error())
		result.Message = shredResultMessage(0, 0, 1)
		return result, nil
	}

	result.ShreddedFiles = 1
	result.FreedSize = size
	result.Message = shredResultMessage(1, size, 0)
	return result, nil
}

func overwriteFile(path string, size int64, passes int) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("打开文件进行覆写失败: %w", err)
	}
	defer file.Close()

	buf := make([]byte, 64*1024)
	for pass := 0; pass < passes; pass++ {
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("覆写前定位失败: %w", err)
		}
		remaining := size
		for remaining > 0 {
			chunkSize := len(buf)
			if remaining < int64(chunkSize) {
				chunkSize = int(remaining)
			}
			if _, err := io.ReadFull(rand.Reader, buf[:chunkSize]); err != nil {
				return fmt.Errorf("生成覆写数据失败: %w", err)
			}
			if _, err := file.Write(buf[:chunkSize]); err != nil {
				return fmt.Errorf("覆写失败: %w", err)
			}
			remaining -= int64(chunkSize)
		}
		if err := file.Sync(); err != nil {
			return fmt.Errorf("刷新覆写数据失败: %w", err)
		}
	}
	return nil
}

func shredResultMessage(shreddedFiles int, freedSize int64, failedFiles int) string {
	return fmt.Sprintf("已粉碎 %d 个文件，释放 %d 字节，失败 %d 个文件。", shreddedFiles, freedSize, failedFiles)
}

func randomSiblingPath(path string) (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "."+hex.EncodeToString(raw)+".tmp"), nil
}

func newShredResult() *model.ShredResult {
	return &model.ShredResult{
		FailedFiles:   make([]string, 0),
		FailedReasons: make(map[string]string),
	}
}

func recordShredFailure(result *model.ShredResult, path, reason string) {
	result.FailedFiles = append(result.FailedFiles, path)
	result.FailedReasons[path] = reason
}
