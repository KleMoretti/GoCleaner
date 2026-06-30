// Package logger stores operation audit records as JSON Lines.
package logger

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gocleaner/internal/model"
	"gocleaner/internal/paths"
)

type Store struct {
	path string
}

const maxJSONLLineBytes = 16 * 1024 * 1024

func DefaultPath() string {
	return paths.OperationLogPath()
}

func New(path string) *Store {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}
	return &Store{path: path}
}

func (s *Store) Append(entry model.OperationLog) error {
	if s == nil {
		return errors.New("日志存储未初始化")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("打开操作日志失败: %w", err)
	}
	defer file.Close()

	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("编码操作日志失败: %w", err)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("写入操作日志失败: %w", err)
	}
	return nil
}

func (s *Store) ReadRecent(limit int) ([]model.OperationLog, error) {
	if s == nil {
		return nil, errors.New("日志存储未初始化")
	}
	if limit <= 0 {
		return []model.OperationLog{}, nil
	}

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.OperationLog{}, nil
		}
		return nil, fmt.Errorf("打开操作日志失败: %w", err)
	}
	defer file.Close()

	var entries []model.OperationLog
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxJSONLLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry model.OperationLog
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取操作日志失败: %w", err)
	}

	reverse(entries)
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func reverse(entries []model.OperationLog) {
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
}
