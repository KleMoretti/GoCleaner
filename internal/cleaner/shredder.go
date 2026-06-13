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
		result.Message = "File shredding requires explicit confirmation."
		return result, ErrHighRiskConfirmationRequired
	}

	path := strings.TrimSpace(request.Path)
	if path == "" {
		recordShredFailure(result, request.Path, "empty path")
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}
	if !allowedShredPasses[request.Passes] {
		recordShredFailure(result, path, "invalid shred passes; allowed values are 1, 3, or 7")
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}

	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			recordShredFailure(result, path, "not found")
		} else {
			recordShredFailure(result, path, classifyAccessError(err))
		}
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		recordShredFailure(result, path, "symbolic link skipped")
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}
	if info.IsDir() {
		recordShredFailure(result, path, "directory cannot be shredded")
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}
	if !info.Mode().IsRegular() {
		recordShredFailure(result, path, "not a regular file")
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}

	size := info.Size()
	if err := overwriteFile(path, size, request.Passes); err != nil {
		recordShredFailure(result, path, err.Error())
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}

	renamedPath, err := randomSiblingPath(path)
	if err != nil {
		recordShredFailure(result, path, "generate random rename failed: "+err.Error())
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}
	if err := os.Rename(path, renamedPath); err != nil {
		recordShredFailure(result, path, "random rename failed: "+err.Error())
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}
	if err := os.Remove(renamedPath); err != nil {
		recordShredFailure(result, renamedPath, "delete after overwrite failed: "+err.Error())
		result.Message = "Shredded 0 file(s), failed 1 file(s)."
		return result, nil
	}

	result.ShreddedFiles = 1
	result.FreedSize = size
	result.Message = fmt.Sprintf("Shredded 1 file(s), freed %d byte(s), failed 0 file(s).", size)
	return result, nil
}

func overwriteFile(path string, size int64, passes int) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open for overwrite failed: %w", err)
	}
	defer file.Close()

	buf := make([]byte, 64*1024)
	for pass := 0; pass < passes; pass++ {
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("seek before overwrite failed: %w", err)
		}
		remaining := size
		for remaining > 0 {
			chunkSize := len(buf)
			if remaining < int64(chunkSize) {
				chunkSize = int(remaining)
			}
			if _, err := io.ReadFull(rand.Reader, buf[:chunkSize]); err != nil {
				return fmt.Errorf("generate overwrite data failed: %w", err)
			}
			if _, err := file.Write(buf[:chunkSize]); err != nil {
				return fmt.Errorf("overwrite failed: %w", err)
			}
			remaining -= int64(chunkSize)
		}
		if err := file.Sync(); err != nil {
			return fmt.Errorf("flush overwrite failed: %w", err)
		}
	}
	return nil
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
