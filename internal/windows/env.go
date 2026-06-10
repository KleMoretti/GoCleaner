// Package windows provides Windows-specific utilities for GoCleaner,
// including environment variable expansion and path resolution.
package windows

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// envVarRe matches Windows-style environment variable references like %TEMP%.
var envVarRe = regexp.MustCompile(`%([^%]+)%`)

// ExpandPath expands environment variables in the given path using Windows %VAR% syntax.
// Supported environment variables include:
//   - %TEMP%
//   - %LOCALAPPDATA%
//   - %APPDATA%
//   - %USERPROFILE%
//   - Any other variable available via os.Getenv
//
// After expansion, the path is cleaned via filepath.Clean.
// If an environment variable is not set, the original reference is left unchanged.
func ExpandPath(path string) string {
	expanded := expandWindowsEnvVars(path)
	return filepath.Clean(expanded)
}

// expandWindowsEnvVars replaces %VAR% patterns with the corresponding environment
// variable value. If a variable is not set, the original %VAR% text is left as-is
// (so callers can detect unresolved references).
func expandWindowsEnvVars(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the variable name without the % delimiters
		varName := match[1 : len(match)-1]
		val, ok := os.LookupEnv(varName)
		if !ok {
			// Variable not set; leave the original text unchanged
			return match
		}
		return val
	})
}

// ExpandPaths expands environment variables in all given paths.
func ExpandPaths(paths []string) []string {
	result := make([]string, len(paths))
	for i, p := range paths {
		result[i] = ExpandPath(p)
	}
	return result
}

// ExpandGlobWildcards expands environment variables in paths that may contain
// glob wildcards (*, ?). The %VAR% portions are expanded first, then the
// path is cleaned while preserving wildcard segments.
func ExpandGlobWildcards(path string) string {
	expanded := expandWindowsEnvVars(path)
	return cleanPreservingWildcards(expanded)
}

// cleanPreservingWildcards cleans the path up to the first wildcard (* or ?),
// preserving everything from that point. This allows paths like
// %LOCALAPPDATA%\Google\Chrome\User Data\*\Cache to be partially cleaned.
func cleanPreservingWildcards(path string) string {
	wildcardIdx := strings.IndexAny(path, "*?")
	if wildcardIdx < 0 {
		return filepath.Clean(path)
	}

	// Split at the first wildcard
	before := path[:wildcardIdx]
	after := path[wildcardIdx:]

	// Clean only the portion before wildcards
	cleaned := filepath.Clean(before)

	// Rejoin, ensuring proper separator between cleaned prefix and wildcard suffix.
	// filepath.Clean strips trailing separators, so we need to add one back
	// if the wildcard suffix starts with a path component (not a separator).
	if len(cleaned) > 0 && !strings.HasSuffix(cleaned, string(filepath.Separator)) {
		if !strings.HasPrefix(after, string(filepath.Separator)) {
			cleaned += string(filepath.Separator)
		}
	}

	return cleaned + after
}

// GetEnvWithDefault returns the value of an environment variable, or the
// default value if the variable is not set or empty.
func GetEnvWithDefault(name, defaultVal string) string {
	val := os.Getenv(name)
	if val == "" {
		return defaultVal
	}
	return val
}

// PathExists checks whether the given path exists on the filesystem.
// Returns false for paths that don't exist or can't be accessed.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ExpandAndResolve expands environment variables and returns the absolute
// path. If the expanded path is relative, it is made absolute.
func ExpandAndResolve(path string) (string, error) {
	expanded := ExpandPath(path)
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("无法解析路径 %q: %w", path, err)
	}
	return abs, nil
}
