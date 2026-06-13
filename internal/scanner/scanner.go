// Package scanner implements the file-system scanning engine for GoCleaner.
// It walks directories, matches files against rule patterns, applies
// age/exclude filters, and produces a flat list of ScanItems for the frontend.
package scanner

import (
	"crypto/md5"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"

	"gocleaner/internal/model"
	"gocleaner/internal/windows"
)

// ── Public API ───────────────────────────────────────────────────────────

type ProgressCallback func(model.ScanProgress)

type ScanOptions struct {
	OnProgress ProgressCallback
}

// Scan runs a full scan using the given rules and returns the results.
// It expands environment variables, resolves glob wildcards in paths,
// walks directories, and applies pattern / exclude / age filters per rule.
//
// Rules that fail validation during loading should already have been
// filtered out by the rules package. This function treats all input
// rules as valid and scannable.
func Scan(rules []model.CleanRule) *model.ScanResult {
	return ScanWithOptions(rules, ScanOptions{})
}

func ScanWithOptions(rules []model.CleanRule, options ScanOptions) *model.ScanResult {
	start := time.Now()
	result := &model.ScanResult{
		Items:  make([]model.ScanItem, 0),
		Errors: make([]model.ScanError, 0),
	}

	for i := range rules {
		rule := &rules[i]
		items, errs := scanRule(rule)
		result.Items = append(result.Items, items...)
		result.Errors = append(result.Errors, errs...)
		emitProgress(options.OnProgress, model.ScanProgress{
			Phase:          model.ScanPhaseScanningFiles,
			CurrentLabel:   rule.Name,
			CompletedSteps: i + 1,
			TotalSteps:     len(rules),
			FoundItems:     len(result.Items),
			FailedItems:    len(result.Errors),
			Percent:        progressPercent(i+1, len(rules)),
		})
	}

	// Compute summary totals from the flat item list.
	for _, item := range result.Items {
		if item.Type == model.TypeFile {
			result.TotalFiles++
			result.TotalSize += item.Size
		}
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

// ── Single-rule scanning ─────────────────────────────────────────────────

// scanRule scans all paths defined in a single rule and returns the
// matching items and any per-path errors encountered.
func scanRule(rule *model.CleanRule) ([]model.ScanItem, []model.ScanError) {
	var items []model.ScanItem
	var errs []model.ScanError

	for _, rawPath := range rule.Paths {
		// Step 1 — expand %ENV_VAR% references and clean the prefix.
		expanded := windows.ExpandGlobWildcards(rawPath)

		// Step 2 — if the path still contains wildcards (* or ?), glob-expand
		// them at the filesystem level.  This is how we turn
		//   …\User Data\*\Cache
		// into concrete profile paths like
		//   …\User Data\Default\Cache
		//   …\User Data\Profile 1\Cache
		hasWildcard := strings.ContainsAny(expanded, "*?")

		var resolvedPaths []string
		if hasWildcard {
			globResults, globErr := filepath.Glob(expanded)
			if globErr != nil {
				errs = append(errs, model.ScanError{
					Path:   rawPath,
					Reason: fmt.Sprintf("通配符路径展开失败: %v", globErr),
				})
				continue
			}
			resolvedPaths = globResults
		} else {
			resolvedPaths = []string{expanded}
		}

		// Step 3 — if nothing was resolved, decide whether that's an error.
		if len(resolvedPaths) == 0 {
			if !hasWildcard {
				// A concrete path that doesn't exist is worth reporting.
				errs = append(errs, model.ScanError{
					Path:   rawPath,
					Reason: "路径不存在",
				})
			}
			// For wildcard paths, "no match" is normal — e.g. Chrome not installed.
			continue
		}

		// Step 4 — scan each resolved path.
		for _, resolvedPath := range resolvedPaths {
			info, statErr := os.Lstat(resolvedPath)
			if statErr != nil {
				errs = append(errs, model.ScanError{
					Path:   resolvedPath,
					Reason: classifyScanAccessError(statErr),
				})
				continue
			}

			// If the resolved path is itself a symlink, skip it.
			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}

			if info.IsDir() {
				walkDir(resolvedPath, rule, &items, &errs)
			} else {
				// A single file (e.g. glob resolved to a concrete file).
				if !info.Mode().IsRegular() {
					continue
				}
				root := filepath.Dir(resolvedPath)
				if item := matchFile(root, resolvedPath, info, rule); item != nil {
					items = append(items, *item)
				}
			}
		}
	}

	return items, errs
}

// ── Directory walk ───────────────────────────────────────────────────────

// walkDir recursively walks a directory tree, matching every regular file
// against the rule.  Symlinks are not followed (filepath.WalkDir does not
// follow directory symlinks, and we skip file symlinks explicitly).
func walkDir(root string, rule *model.CleanRule, items *[]model.ScanItem, errs *[]model.ScanError) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			*errs = append(*errs, model.ScanError{
				Path:   path,
				Reason: classifyScanAccessError(walkErr),
			})
			return nil // keep walking siblings
		}

		// Directories are skipped — only files become ScanItems.
		if d.IsDir() {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			*errs = append(*errs, model.ScanError{
				Path:   path,
				Reason: classifyScanAccessError(infoErr),
			})
			return nil
		}

		// Explicitly skip symlinks and non-regular files.
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		if item := matchFile(root, path, info, rule); item != nil {
			*items = append(*items, *item)
		}

		return nil
	})
}

// ── File matching ────────────────────────────────────────────────────────

// matchFile checks whether a single file matches the rule's criteria:
// patterns, exclude list, and minimum-age filter.  Returns a ScanItem
// if everything passes, or nil if the file should be skipped.
func matchFile(root, filePath string, info os.FileInfo, rule *model.CleanRule) *model.ScanItem {
	name := info.Name()
	relPath := relativePath(root, filePath)

	// ── Pattern matching ──────────────────────────────────────────────
	// Empty patterns list means "all files in the directory".
	if len(rule.Patterns) > 0 {
		matched := false
		for _, pattern := range rule.Patterns {
			if globMatch(pattern, name) || globMatch(pattern, relPath) {
				matched = true
				break
			}
		}
		if !matched {
			return nil
		}
	}

	// ── Exclude matching ──────────────────────────────────────────────
	// A file is excluded if its name matches an exclude glob OR if its
	// path under the scan root matches an exclude path / segment.
	for _, exclude := range rule.Exclude {
		if shouldExclude(exclude, name, relPath) {
			return nil
		}
	}

	// ── Minimum-age filter ────────────────────────────────────────────
	if rule.MinAgeDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -rule.MinAgeDays)
		if info.ModTime().After(cutoff) {
			return nil
		}
	}

	// ── Selected state ────────────────────────────────────────────────
	selected := rule.DefaultOn
	// Safety net: high-risk items are never pre-selected, even if the
	// rule (erroneously) has default_on=true.  The rules loader already
	// forces high-risk rule DefaultOn to false, but we double-check here.
	if rule.Risk == model.RiskHigh {
		selected = false
	}

	return &model.ScanItem{
		ID:           generateID(rule.Category, filePath),
		Path:         filePath,
		Name:         name,
		Type:         model.TypeFile,
		Category:     rule.Category,
		Size:         info.Size(),
		Risk:         rule.Risk,
		Source:       rule.Name,
		LastModified: info.ModTime().Unix(),
		Selected:     selected,
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────

// generateID creates a short, unique identifier for a scan item based on
// its category and absolute path.  The format is "{category}_{hash8}".
func generateID(category, path string) string {
	hash := md5.Sum([]byte(path))
	return fmt.Sprintf("%s_%x", category, hash[:8])
}

func globMatch(pattern, value string) bool {
	if ok, err := filepath.Match(pattern, value); err == nil && ok {
		return true
	}
	if ok, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(value)); err == nil && ok {
		return true
	}

	slashPattern := filepath.ToSlash(pattern)
	slashValue := filepath.ToSlash(value)
	if ok, err := pathpkg.Match(slashPattern, slashValue); err == nil && ok {
		return true
	}
	if ok, err := pathpkg.Match(strings.ToLower(slashPattern), strings.ToLower(slashValue)); err == nil && ok {
		return true
	}

	return false
}

func shouldExclude(exclude, name, relPath string) bool {
	exclude = strings.TrimSpace(exclude)
	if exclude == "" {
		return false
	}

	if globMatch(exclude, name) || globMatch(exclude, relPath) {
		return true
	}

	normExclude := normalizeRelPath(exclude)
	normRel := normalizeRelPath(relPath)
	if normExclude == "" || normRel == "" {
		return false
	}

	if strings.Contains(normExclude, "/") {
		return normRel == normExclude ||
			strings.HasPrefix(normRel, normExclude+"/") ||
			strings.Contains(normRel, "/"+normExclude+"/") ||
			strings.HasSuffix(normRel, "/"+normExclude)
	}

	for _, segment := range strings.Split(normRel, "/") {
		if segment == normExclude {
			return true
		}
	}
	return false
}

func relativePath(root, filePath string) string {
	rel, err := filepath.Rel(root, filePath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Base(filePath)
	}
	return rel
}

func normalizeRelPath(value string) string {
	value = strings.TrimSpace(value)
	value = filepath.ToSlash(value)
	value = strings.Trim(value, "/")
	return strings.ToLower(value)
}

func emitProgress(callback ProgressCallback, progress model.ScanProgress) {
	if callback != nil {
		callback(progress)
	}
}

func progressPercent(completed, total int) int {
	if total <= 0 {
		return 100
	}
	percent := completed * 100 / total
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func classifyScanAccessError(err error) string {
	message := err.Error()
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "being used by another process"),
		strings.Contains(lower, "process cannot access"),
		strings.Contains(lower, "sharing violation"),
		strings.Contains(lower, "file is locked"):
		return "文件被占用: " + message
	case strings.Contains(lower, "access is denied"),
		strings.Contains(lower, "permission denied"):
		return "权限不足: " + message
	default:
		return "访问失败: " + message
	}
}
