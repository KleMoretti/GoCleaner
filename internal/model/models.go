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
	CategoryRegistry = "registry"
)

// Operation type constants for logging.
const (
	OpScan           = "scan"
	OpClean          = "clean"
	OpShred          = "shred"
	OpRegistryBackup = "registry_backup"
	OpRegistryDelete = "registry_delete"
	OpQuarantine     = "quarantine"
	OpRestore        = "restore"
)

// Item type constants.
const (
	TypeFile      = "file"
	TypeDirectory = "directory"
	TypeRegistry  = "registry"
	TypePlugin    = "plugin"
)

// Scan progress phases emitted to the frontend.
const (
	ScanPhaseLoadingRules     = "loading_rules"
	ScanPhaseScanningFiles    = "scanning_files"
	ScanPhaseScanningPlugins  = "scanning_plugins"
	ScanPhaseScanningRegistry = "scanning_registry"
	ScanPhaseDone             = "done"
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
	ID           string        `json:"id"`                 // Unique identifier: {category}_{path_hash}
	Path         string        `json:"path"`               // Absolute path to the file/item
	Name         string        `json:"name"`               // File name or display name
	Type         string        `json:"type"`               // Type: file, directory, registry, plugin
	Category     string        `json:"category"`           // Category from the parent rule
	Size         int64         `json:"size"`               // File size in bytes (0 for non-files)
	Risk         string        `json:"risk"`               // Risk level: low, medium, high
	Source       string        `json:"source"`             // Name of the source rule
	LastModified int64         `json:"last_modified"`      // Last modification time as Unix timestamp
	Selected     bool          `json:"selected"`           // Whether this item is currently selected for cleanup
	Plugin       *PluginInfo   `json:"plugin,omitempty"`   // Manifest metadata for browser plugins
	Registry     *RegistryInfo `json:"registry,omitempty"` // Registry metadata for invalid registry values
}

// PluginInfo contains browser extension metadata read from manifest.json.
type PluginInfo struct {
	Browser      string `json:"browser"`
	Profile      string `json:"profile"`
	ExtensionID  string `json:"extension_id"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	ManifestPath string `json:"manifest_path"`
}

// QuarantineRecord describes a plugin directory moved into quarantine.
type QuarantineRecord struct {
	RecordID       string `json:"record_id"`
	OriginalPath   string `json:"original_path"`
	QuarantinePath string `json:"quarantine_path"`
	Name           string `json:"name"`
	ItemType       string `json:"item_type"`
	Browser        string `json:"browser"`
	CreatedAt      string `json:"created_at"`
	Size           int64  `json:"size"`
	RestoredAt     string `json:"restored_at,omitempty"`
}

// QuarantineResult summarizes plugin quarantine or restore operations.
type QuarantineResult struct {
	MovedItems    int               `json:"moved_items"`
	RestoredItems int               `json:"restored_items"`
	FailedItems   []string          `json:"failed_items"`
	FailedReasons map[string]string `json:"failed_reasons"`
	Message       string            `json:"message"`
}

// RegistryInfo describes one invalid registry value found during registry scanning.
type RegistryInfo struct {
	Hive         string `json:"hive"`
	KeyPath      string `json:"key_path"`
	ValueName    string `json:"value_name"`
	ValueType    string `json:"value_type"`
	RawData      string `json:"raw_data"`
	ExpandedPath string `json:"expanded_path"`
	TargetPath   string `json:"target_path"`
	BackupPath   string `json:"backup_path"`
}

// RegistryActionResult summarizes backup and deletion of selected registry values.
type RegistryActionResult struct {
	DeletedValues int               `json:"deleted_values"`
	BackupPath    string            `json:"backup_path"`
	FailedItems   []string          `json:"failed_items"`
	FailedReasons map[string]string `json:"failed_reasons"`
	Message       string            `json:"message"`
}

// ShredRequest describes one explicit user-selected file shred operation.
type ShredRequest struct {
	Path   string `json:"path"`
	Passes int    `json:"passes"`
}

// ShredResult summarizes file shredding results.
type ShredResult struct {
	ShreddedFiles int               `json:"shredded_files"`
	FreedSize     int64             `json:"freed_size"`
	FailedFiles   []string          `json:"failed_files"`
	FailedReasons map[string]string `json:"failed_reasons"`
	Message       string            `json:"message"`
}

// ScanError records a path that could not be scanned and the reason why.
type ScanError struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// ScanResult holds the complete outcome of a scan operation.
type ScanResult struct {
	Items      []ScanItem  `json:"items"`       // All discovered scan items
	TotalFiles int         `json:"total_files"` // Number of files found
	TotalSize  int64       `json:"total_size"`  // Total bytes across all files
	Errors     []ScanError `json:"errors"`      // Paths that failed during scanning
	Duration   int64       `json:"duration_ms"` // Scan duration in milliseconds
}

// ScanProgress describes coarse-grained scan progress for UI feedback.
type ScanProgress struct {
	Phase          string `json:"phase"`
	CurrentLabel   string `json:"current_label"`
	CompletedSteps int    `json:"completed_steps"`
	TotalSteps     int    `json:"total_steps"`
	FoundItems     int    `json:"found_items"`
	FailedItems    int    `json:"failed_items"`
	Percent        int    `json:"percent"`
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
