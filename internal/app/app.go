// Package app provides the Wails binding layer for GoCleaner.
// It bridges the frontend UI with the backend logic (rules, scanner, cleaner).
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gocleaner/internal/cleaner"
	"gocleaner/internal/logger"
	"gocleaner/internal/model"
	"gocleaner/internal/rules"
	"gocleaner/internal/scanner"
	"gocleaner/internal/windows"
)

// App is the main application struct. Its methods are automatically
// bound to the Wails frontend via the Bind option in main.go.
type App struct {
	ctx           context.Context
	embeddedRules []byte // Embedded fallback rules from the binary
}

// New creates a new App instance. embeddedRules is the content of the
// default cleaner_rules.json embedded into the binary at build time.
func New(embeddedRules []byte) *App {
	return &App{
		embeddedRules: embeddedRules,
	}
}

// Startup is called by Wails when the application starts.
// It saves the context for use by other methods.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// resolveRulesPath finds the cleaner_rules.json file by checking:
//  1. <exe_dir>/configs/cleaner_rules.json (for packaged builds)
//  2. configs/cleaner_rules.json (working directory, for dev)
//
// Returns the path if found, or an empty string if neither exists.
func resolveRulesPath() string {
	// 1. Try alongside the executable (packaged distribution)
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "configs", "cleaner_rules.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// 2. Try working directory (development)
	if _, err := os.Stat("configs/cleaner_rules.json"); err == nil {
		return "configs/cleaner_rules.json"
	}

	return ""
}

// =========================================================================
// Scan API — Day 5-7: full file-system scanning
// =========================================================================

// Scan loads all cleaning rules and executes a full file-system scan,
// returning the complete scan result (items, totals, errors, duration).
//
// Rules are loaded from the on-disk config file (or embedded defaults).
// The scanner expands environment variables, resolves glob wildcards,
// walks directory trees, and applies pattern / exclude / age filters.
//
// High-risk items are never pre-selected, even if a rule has default_on=true.
func (a *App) Scan() (*model.ScanResult, error) {
	start := time.Now()
	rulesList, err := a.GetRulesPreview()
	if err != nil {
		return nil, fmt.Errorf("加载规则失败: %w", err)
	}

	if len(rulesList) == 0 {
		return nil, fmt.Errorf("没有可用的扫描规则")
	}

	result := scanner.Scan(rulesList)
	pluginItems, pluginErrors := scanner.ScanBrowserPlugins(scanner.DefaultPluginTargets())
	result.Items = append(result.Items, pluginItems...)
	result.Errors = append(result.Errors, pluginErrors...)
	for _, item := range pluginItems {
		result.TotalSize += item.Size
	}
	result.Duration = time.Since(start).Milliseconds()
	if err := appendScanLog(result); err != nil {
		return result, fmt.Errorf("record scan operation log: %w", err)
	}
	return result, nil
}

// Clean executes deletion for frontend-selected scan items.
func (a *App) Clean(items []model.ScanItem, highRiskConfirmed bool) (*model.CleanResult, error) {
	start := time.Now()
	result, cleanErr := cleaner.Clean(items, cleaner.Options{
		HighRiskConfirmed: highRiskConfirmed,
	})
	if result == nil {
		result = &model.CleanResult{
			FailedFiles:   make([]string, 0),
			FailedReasons: make(map[string]string),
		}
	}

	if err := appendCleanLog(result, time.Since(start).Milliseconds()); err != nil {
		if cleanErr != nil {
			return result, fmt.Errorf("%w; record clean operation log: %v", cleanErr, err)
		}
		return result, fmt.Errorf("record clean operation log: %w", err)
	}

	return result, cleanErr
}

// QuarantinePlugins moves selected plugin scan items into the quarantine area.
func (a *App) QuarantinePlugins(items []model.ScanItem) (*model.QuarantineResult, error) {
	start := time.Now()
	store := cleaner.NewQuarantineStore(cleaner.DefaultQuarantineRoot())
	result, quarantineErr := store.QuarantinePlugins(items)
	if result == nil {
		result = &model.QuarantineResult{
			FailedItems:   make([]string, 0),
			FailedReasons: make(map[string]string),
		}
	}

	if err := appendQuarantineLog(model.OpQuarantine, result, time.Since(start).Milliseconds()); err != nil {
		if quarantineErr != nil {
			return result, fmt.Errorf("%w; record quarantine operation log: %v", quarantineErr, err)
		}
		return result, fmt.Errorf("record quarantine operation log: %w", err)
	}
	return result, quarantineErr
}

// ListQuarantineRecords returns plugin quarantine records.
func (a *App) ListQuarantineRecords() ([]model.QuarantineRecord, error) {
	return cleaner.NewQuarantineStore(cleaner.DefaultQuarantineRoot()).ListRecords()
}

// RestoreQuarantinedPlugin restores one plugin from quarantine.
func (a *App) RestoreQuarantinedPlugin(recordID string) (*model.QuarantineResult, error) {
	start := time.Now()
	store := cleaner.NewQuarantineStore(cleaner.DefaultQuarantineRoot())
	result, restoreErr := store.RestorePlugin(recordID)
	if result == nil {
		result = &model.QuarantineResult{
			FailedItems:   make([]string, 0),
			FailedReasons: make(map[string]string),
		}
	}
	if err := appendQuarantineLog(model.OpRestore, result, time.Since(start).Milliseconds()); err != nil {
		if restoreErr != nil {
			return result, fmt.Errorf("%w; record restore operation log: %v", restoreErr, err)
		}
		return result, fmt.Errorf("record restore operation log: %w", err)
	}
	return result, restoreErr
}

// GetOperationLogs returns the newest operation log entries first.
func (a *App) GetOperationLogs(limit int) ([]model.OperationLog, error) {
	return logger.New(logger.DefaultPath()).ReadRecent(limit)
}

// Ping is a health-check method to verify the Go backend is reachable.
func (a *App) Ping() string {
	return "GoCleaner backend is running"
}

// GetEnvInfo returns the expanded values of common Windows environment
// variables used in cleaning rules. This lets the frontend verify that
// path expansion is working correctly.
func (a *App) GetEnvInfo() map[string]string {
	vars := []string{"TEMP", "LOCALAPPDATA", "APPDATA", "USERPROFILE"}
	result := make(map[string]string, len(vars))
	for _, v := range vars {
		result[v] = windows.ExpandPath("%" + v + "%")
	}
	return result
}

// GetRulesPreview loads the cleaning rules and returns a summary suitable
// for display in the frontend. This is a placeholder that will evolve
// into the full scan flow in Days 5-7.
//
// It first tries the on-disk config file, then falls back to the embedded
// default rules baked into the binary.
func (a *App) GetRulesPreview() ([]model.CleanRule, error) {
	result, err := a.loadRules()
	if err != nil {
		return nil, fmt.Errorf("加载规则文件失败: %w", err)
	}

	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			println("规则加载错误:", e.Error())
		}
	}
	if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			println("规则加载警告:", w.Error())
		}
	}

	return result.Rules, nil
}

// GetRulesWarnings returns validation warnings for rules that are still loaded.
func (a *App) GetRulesWarnings() ([]string, error) {
	result, err := a.loadRules()
	if err != nil {
		return nil, fmt.Errorf("加载规则文件失败: %w", err)
	}

	warnings := make([]string, 0, len(result.Warnings))
	for _, w := range result.Warnings {
		warnings = append(warnings, w.Error())
	}
	return warnings, nil
}

// loadRules tries the on-disk config first, then falls back to embedded rules.
func (a *App) loadRules() (*rules.LoadResult, error) {
	path := resolveRulesPath()
	if path != "" {
		return rules.LoadFromFile(path)
	}

	// Fallback: parse the embedded rules baked into the binary
	if len(a.embeddedRules) > 0 {
		return rules.LoadFromBytes(a.embeddedRules)
	}

	return nil, fmt.Errorf("未找到规则配置文件，且二进制中没有嵌入默认规则")
}

// GetRuleCategories returns the distinct categories present in the loaded rules.
func (a *App) GetRuleCategories() ([]string, error) {
	rulesList, err := a.GetRulesPreview()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var categories []string
	for _, r := range rulesList {
		if !seen[r.Category] {
			seen[r.Category] = true
			categories = append(categories, r.Category)
		}
	}
	return categories, nil
}

func appendScanLog(result *model.ScanResult) error {
	entry := model.NewOperationLog(model.OpScan)
	entry.ScannedFiles = result.TotalFiles
	entry.Duration = result.Duration
	for _, scanErr := range result.Errors {
		entry.FailedPaths = append(entry.FailedPaths, scanErr.Path)
		entry.FailedReasons = append(entry.FailedReasons, scanErr.Reason)
	}
	return logger.New(logger.DefaultPath()).Append(*entry)
}

func appendCleanLog(result *model.CleanResult, duration int64) error {
	entry := model.NewOperationLog(model.OpClean)
	entry.DeletedFiles = result.DeletedFiles
	entry.FreedSize = result.FreedSize
	entry.FailedPaths = append(entry.FailedPaths, result.FailedFiles...)
	for _, path := range result.FailedFiles {
		entry.FailedReasons = append(entry.FailedReasons, result.FailedReasons[path])
	}
	entry.Duration = duration
	return logger.New(logger.DefaultPath()).Append(*entry)
}

func appendQuarantineLog(opType string, result *model.QuarantineResult, duration int64) error {
	entry := model.NewOperationLog(opType)
	switch opType {
	case model.OpRestore:
		entry.DeletedFiles = result.RestoredItems
	default:
		entry.DeletedFiles = result.MovedItems
	}
	entry.FailedPaths = append(entry.FailedPaths, result.FailedItems...)
	for _, path := range result.FailedItems {
		entry.FailedReasons = append(entry.FailedReasons, result.FailedReasons[path])
	}
	entry.Duration = duration
	return logger.New(logger.DefaultPath()).Append(*entry)
}
