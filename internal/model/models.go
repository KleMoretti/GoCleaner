// Package model defines the core data structures used throughout GoCleaner.
package model

import "time"

// Risk levels for scan items and rules.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// ValidRiskLevels is the set of allowed risk level values.
var ValidRiskLevels = map[string]bool{
	RiskLow:    true,
	RiskMedium: true,
	RiskHigh:   true,
}

// Category constants for organizing rules and scan items.
const (
	CategorySystem   = "system"
	CategorySoftware = "software"
	CategoryPrivacy  = "privacy"
	CategoryPlugin   = "plugin"
)

// Operation type constants for logging.
const (
	OpScan           = "scan"
	OpClean          = "clean"
	OpShred          = "shred"
	OpRegistryBackup = "registry_backup"
	OpRegistryDelete = "registry_delete"
)

// Item type constants.
const (
	TypeFile      = "file"
	TypeDirectory = "directory"
	TypeRegistry  = "registry"
	TypePlugin    = "plugin"
)

// CleanRule defines a single cleaning rule loaded from the JSON configuration.
// Each rule specifies what to scan, where to look, and how risky the cleanup is.
type CleanRule struct {
	Name       string   `json:"name"`         // Human-readable rule name, e.g. "Chrome Cache"
	Category   string   `json:"category"`     // Category: system, software, privacy, plugin
	Paths      []string `json:"paths"`        // Scan paths with optional env vars like %TEMP%
	Patterns   []string `json:"patterns"`     // File matching globs like ["*.log", "*.tmp"]; empty = all files
	Exclude    []string `json:"exclude"`      // Sub-paths or patterns to exclude from scanning
	Risk       string   `json:"risk"`         // Risk level: low, medium, high
	MinAgeDays int      `json:"min_age_days"` // Minimum file age in days; 0 = no age filter
	DefaultOn  bool     `json:"default_on"`   // Whether this rule's items are checked by default

	// Errors contains any validation error for this rule (populated during loading, not serialized).
	Errors []string `json:"-"`
}

// ScanItem represents a single scannable item discovered during scanning.
// It may be a file, directory, registry entry, or plugin.
type ScanItem struct {
	ID           string `json:"id"`            // Unique identifier: {category}_{path_hash}
	Path         string `json:"path"`          // Absolute path to the file/item
	Name         string `json:"name"`          // File name or display name
	Type         string `json:"type"`          // Type: file, directory, registry, plugin
	Category     string `json:"category"`      // Category from the parent rule
	Size         int64  `json:"size"`          // File size in bytes (0 for non-files)
	Risk         string `json:"risk"`          // Risk level: low, medium, high
	Source       string `json:"source"`        // Name of the source rule
	LastModified int64  `json:"last_modified"` // Last modification time as Unix timestamp
	Selected     bool   `json:"selected"`      // Whether this item is currently selected for cleanup
}

// CleanResult summarizes the outcome of a cleaning operation.
type CleanResult struct {
	DeletedFiles  int               `json:"deleted_files"`  // Number of successfully deleted files
	FreedSize     int64             `json:"freed_size"`     // Total bytes freed
	FailedFiles   []string          `json:"failed_files"`   // Paths that could not be deleted
	FailedReasons map[string]string `json:"failed_reasons"` // Failed path → reason mapping
	Message       string            `json:"message"`        // Human-readable summary
}

// OperationLog records a single operation (scan, clean, shred, etc.) for auditing.
// Stored as JSONL in data/operation.jsonl.
type OperationLog struct {
	Timestamp     string   `json:"timestamp"`      // ISO 8601 timestamp
	Operation     string   `json:"operation"`      // Operation type: scan, clean, shred, etc.
	ScannedFiles  int      `json:"scanned_files"`  // Total files scanned (for scan ops)
	DeletedFiles  int      `json:"deleted_files"`  // Total files deleted (for clean ops)
	FreedSize     int64    `json:"freed_size"`     // Bytes freed
	FailedPaths   []string `json:"failed_paths"`   // Paths that failed
	FailedReasons []string `json:"failed_reasons"` // Reasons for each failure
	Duration      int64    `json:"duration"`       // Operation duration in milliseconds
}

// NewOperationLog creates a new OperationLog with the current timestamp.
func NewOperationLog(opType string) *OperationLog {
	return &OperationLog{
		Timestamp: time.Now().Format(time.RFC3339),
		Operation: opType,
	}
}
