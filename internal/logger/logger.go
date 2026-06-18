// Package logger stores operation audit records as JSON Lines.
package logger

import (
	"bufio"
	"encoding/json"
	"errors"
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
		return errors.New("logger store is nil")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *Store) ReadRecent(limit int) ([]model.OperationLog, error) {
	if s == nil {
		return nil, errors.New("logger store is nil")
	}
	if limit <= 0 {
		return []model.OperationLog{}, nil
	}

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.OperationLog{}, nil
		}
		return nil, err
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
		return nil, err
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
