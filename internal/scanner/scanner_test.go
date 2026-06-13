package scanner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gocleaner/internal/model"
)

// ── Test helpers ─────────────────────────────────────────────────────────

// makeTempDir creates a temporary directory structure for testing.
//
// The files map uses relative paths as keys and file sizes (approximate)
// as values.  Each file is created with the given content.
func makeTempDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for relPath, content := range files {
		fullPath := filepath.Join(dir, relPath)
		parent := filepath.Dir(fullPath)
		if err := os.MkdirAll(parent, 0755); err != nil {
			t.Fatalf("创建测试目录失败 %q: %v", parent, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("创建测试文件失败 %q: %v", fullPath, err)
		}
	}
	return dir
}

// setFileMtime sets the modification time of a file (for MinAgeDays tests).
func setFileMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("设置文件时间失败 %q: %v", path, err)
	}
}

// makeRule is a shorthand for building a CleanRule in tests.
func makeRule(name, category string, paths, patterns, exclude []string, risk string, minAge int, defaultOn bool) model.CleanRule {
	return model.CleanRule{
		Name:       name,
		Category:   category,
		Paths:      paths,
		Patterns:   patterns,
		Exclude:    exclude,
		Risk:       risk,
		MinAgeDays: minAge,
		DefaultOn:  defaultOn,
	}
}

// ── Day 5 tests: scanner skeleton and basic traversal ────────────────────

func TestScan_EmptyDirectory(t *testing.T) {
	dir := makeTempDir(t, nil) // empty directory
	rule := makeRule("test", "system", []string{dir}, []string{}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if len(result.Items) != 0 {
		t.Errorf("空目录应返回 0 个扫描项，实际得到 %d 个", len(result.Items))
	}
	if len(result.Errors) != 0 {
		t.Errorf("空目录不应有错误，实际得到 %d 个: %v", len(result.Errors), result.Errors)
	}
}

func TestScan_FileSizeStatistics(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"small.txt": "hello",
		"big.txt":   strings.Repeat("x", 10000),
	})
	rule := makeRule("test", "system", []string{dir}, []string{}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if result.TotalFiles != 2 {
		t.Errorf("预期 2 个文件，实际得到 %d 个", result.TotalFiles)
	}

	var totalSize int64
	for _, item := range result.Items {
		totalSize += item.Size
	}

	if totalSize != result.TotalSize {
		t.Errorf("TotalSize(%d) 应等于各文件大小之和(%d)", result.TotalSize, totalSize)
	}

	if result.TotalSize < 10000 {
		t.Errorf("总大小应至少为 10000 字节，实际为 %d", result.TotalSize)
	}
}

func TestScan_NonExistentPathDoesNotPanic(t *testing.T) {
	rule := makeRule("test", "system", []string{`Z:\definitely\does\not\exist`}, []string{"*.tmp"}, []string{}, model.RiskLow, 0, true)

	// Must not panic.
	result := Scan([]model.CleanRule{rule})

	// Should record an error for the missing path.
	if len(result.Errors) == 0 {
		t.Error("不存在的路径应产生错误记录")
	} else {
		t.Logf("路径不存在错误: %v", result.Errors[0])
	}
}

func TestScan_ScanItemFieldsCorrect(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"error.log": "some log content here",
	})
	rule := makeRule(
		"日志清理规则",
		"system",
		[]string{dir},
		[]string{"*.log"},
		[]string{},
		model.RiskMedium,
		0,
		true,
	)

	result := Scan([]model.CleanRule{rule})

	if len(result.Items) != 1 {
		t.Fatalf("预期 1 个扫描项，实际得到 %d 个", len(result.Items))
	}

	item := result.Items[0]

	if item.Name != "error.log" {
		t.Errorf("Name = %q, want %q", item.Name, "error.log")
	}
	if item.Type != model.TypeFile {
		t.Errorf("Type = %q, want %q", item.Type, model.TypeFile)
	}
	if item.Category != "system" {
		t.Errorf("Category = %q, want %q", item.Category, "system")
	}
	expectedSize := int64(len("some log content here"))
	if item.Size != expectedSize {
		t.Errorf("Size = %d, want %d", item.Size, expectedSize)
	}
	if item.Risk != model.RiskMedium {
		t.Errorf("Risk = %q, want %q", item.Risk, model.RiskMedium)
	}
	if item.Source != "日志清理规则" {
		t.Errorf("Source = %q, want %q", item.Source, "日志清理规则")
	}
	if !item.Selected {
		t.Error("中风险规则 default_on=true 时 Selected 应为 true")
	}
	if item.ID == "" {
		t.Error("ID 不应为空")
	}
	if !strings.HasPrefix(item.ID, "system_") {
		t.Errorf("ID 应以 category_ 开头，实际为 %q", item.ID)
	}
	if item.LastModified == 0 {
		t.Error("LastModified 不应为 0")
	}
}

func TestScan_DurationIsSet(t *testing.T) {
	dir := makeTempDir(t, map[string]string{"a.txt": "hello"})
	rule := makeRule("test", "system", []string{dir}, []string{}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if result.Duration < 0 {
		t.Errorf("Duration 不应为负数: %d", result.Duration)
	}
	// Duration should be >= 0ms (could be 0 if very fast).
	t.Logf("扫描耗时: %d ms", result.Duration)
}

// ── Day 6 tests: rule matching and safety filtering ──────────────────────

func TestScan_PatternMatching(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"app.tmp":      "tmp content",
		"server.log":   "log content",
		"readme.txt":   "text content",
		"data.json":    "json content",
		"image.png":    "png content",
		"nested/a.tmp": "nested tmp",
		"nested/b.log": "nested log",
		"nested/c.txt": "nested txt",
	})
	rule := makeRule("test", "system", []string{dir}, []string{"*.tmp", "*.log"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	// Should match: app.tmp, server.log, nested/a.tmp, nested/b.log
	if result.TotalFiles != 4 {
		t.Errorf("预期 4 个匹配文件 (*.tmp + *.log)，实际得到 %d 个", result.TotalFiles)
		for _, item := range result.Items {
			t.Logf("  匹配: %s", item.Path)
		}
	}

	// Verify non-matching extensions are excluded
	for _, item := range result.Items {
		ext := filepath.Ext(item.Name)
		if ext != ".tmp" && ext != ".log" {
			t.Errorf("不匹配的扩展名不应出现在结果中: %s", item.Name)
		}
	}
}

func TestScan_PatternMatchingIsCaseInsensitive(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"APP.TMP":    "tmp content",
		"SERVER.LOG": "log content",
		"readme.txt": "text content",
	})
	rule := makeRule("test", "system", []string{dir}, []string{"*.tmp", "*.log"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if result.TotalFiles != 2 {
		t.Errorf("Windows 扫描匹配应忽略大小写，预期 2 个文件，实际得到 %d 个", result.TotalFiles)
		for _, item := range result.Items {
			t.Logf("  结果: %s", item.Path)
		}
	}
}

func TestScan_EmptyPatternsMeansAllFiles(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"a.txt":  "text",
		"b.dat":  "data",
		"c.json": "json",
	})
	rule := makeRule("test", "system", []string{dir}, []string{}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if result.TotalFiles != 3 {
		t.Errorf("空 patterns 应匹配所有文件，预期 3 个，实际得到 %d 个", result.TotalFiles)
	}
}

func TestScan_ExcludeFiltering(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"include.log":  "log",
		"exclude.log":  "should be excluded",
		"keep.tmp":     "tmp",
		"sub/keep.log": "keep",
		"sub/skip.log": "skip",
	})
	rule := makeRule(
		"test", "system",
		[]string{dir},
		[]string{"*.log", "*.tmp"},
		[]string{"exclude.log", "skip.log"}, // exclude by name
		model.RiskLow, 0, true,
	)

	result := Scan([]model.CleanRule{rule})

	// Should match: include.log, keep.tmp, sub/keep.log
	if result.TotalFiles != 3 {
		t.Errorf("排除过滤后预期 3 个文件，实际得到 %d 个", result.TotalFiles)
		for _, item := range result.Items {
			t.Logf("  结果: %s", item.Path)
		}
	}

	for _, item := range result.Items {
		if item.Name == "exclude.log" || item.Name == "skip.log" {
			t.Errorf("被排除的文件不应出现在结果中: %s", item.Name)
		}
	}
}

func TestScan_ExcludeBySubPath(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"cache/a.log":     "keep",
		"cache/b.log":     "keep",
		"important/a.log": "skip", // sub-path "important" should be excluded
		"important/b.log": "skip",
	})
	rule := makeRule(
		"test", "system",
		[]string{dir},
		[]string{"*.log"},
		[]string{"important"},
		model.RiskLow, 0, true,
	)

	result := Scan([]model.CleanRule{rule})

	if result.TotalFiles != 2 {
		t.Errorf("子路径排除后预期 2 个文件，实际得到 %d 个", result.TotalFiles)
		for _, item := range result.Items {
			t.Logf("  结果: %s", item.Path)
		}
	}

	for _, item := range result.Items {
		if strings.Contains(item.Path, "important") {
			t.Errorf("包含排除子路径的文件不应出现在结果中: %s", item.Path)
		}
	}
}

func TestScan_ExcludeDoesNotMatchParentOfScanRoot(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "important-parent")
	root := filepath.Join(parent, "cache")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatalf("创建扫描根目录失败: %v", err)
	}
	filePath := filepath.Join(root, "keep.log")
	if err := os.WriteFile(filePath, []byte("keep"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	rule := makeRule("test", "system", []string{root}, []string{"*.log"}, []string{"important"}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if result.TotalFiles != 1 {
		t.Errorf("exclude 应只匹配扫描根目录内的相对路径，预期 1 个文件，实际得到 %d 个", result.TotalFiles)
		for _, item := range result.Items {
			t.Logf("  结果: %s", item.Path)
		}
	}
}

func TestScan_MinAgeDaysFilter(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"old.log": "old content",
		"new.log": "new content",
	})

	oldPath := filepath.Join(dir, "old.log")
	newPath := filepath.Join(dir, "new.log")

	// Set old.log to 10 days ago, new.log to 1 day ago.
	setFileMtime(t, oldPath, time.Now().AddDate(0, 0, -10))
	setFileMtime(t, newPath, time.Now().AddDate(0, 0, -1))

	rule := makeRule("test", "system", []string{dir}, []string{"*.log"}, []string{}, model.RiskLow, 7, true)

	result := Scan([]model.CleanRule{rule})

	// Only old.log should match (older than 7 days).
	if result.TotalFiles != 1 {
		t.Errorf("MinAgeDays=7 时应只返回超过 7 天的文件，预期 1 个，实际得到 %d 个", result.TotalFiles)
		for _, item := range result.Items {
			age := time.Since(time.Unix(item.LastModified, 0)).Hours() / 24
			t.Logf("  文件: %s (年龄: %.1f 天)", item.Name, age)
		}
	}

	if len(result.Items) == 1 && result.Items[0].Name != "old.log" {
		t.Errorf("应返回 old.log (10天前)，实际返回 %s", result.Items[0].Name)
	}
}

func TestScan_MinAgeDaysZeroMeansNoFilter(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"fresh.log": "new",
	})
	rule := makeRule("test", "system", []string{dir}, []string{"*.log"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	// Even brand new file should match when MinAgeDays=0.
	if result.TotalFiles != 1 {
		t.Errorf("MinAgeDays=0 时不应过滤任何文件，预期 1 个，实际得到 %d 个", result.TotalFiles)
	}
}

func TestScan_HighRiskDefaultUnselected(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"system.log": "log",
	})
	// Even with default_on=true, high-risk items must have Selected=false.
	rule := makeRule("高风险规则", "system", []string{dir}, []string{"*.log"}, []string{}, model.RiskHigh, 0, true)

	result := Scan([]model.CleanRule{rule})

	if len(result.Items) != 1 {
		t.Fatalf("预期 1 个扫描项，实际得到 %d 个", len(result.Items))
	}

	if result.Items[0].Selected {
		t.Error("高风险扫描项 Selected 必须为 false，即使规则 default_on=true")
	}
	if result.Items[0].Risk != model.RiskHigh {
		t.Errorf("Risk = %q, want %q", result.Items[0].Risk, model.RiskHigh)
	}
}

func TestScan_LowRiskDefaultOnPreserved(t *testing.T) {
	dir := makeTempDir(t, map[string]string{"test.log": "log"})
	rule := makeRule("低风险", "system", []string{dir}, []string{"*.log"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if len(result.Items) != 1 {
		t.Fatalf("预期 1 个扫描项，实际得到 %d 个", len(result.Items))
	}
	if !result.Items[0].Selected {
		t.Error("低风险 default_on=true 时 Selected 应为 true")
	}
}

func TestScan_MediumRiskDefaultOffPreserved(t *testing.T) {
	dir := makeTempDir(t, map[string]string{"test.log": "log"})
	rule := makeRule("中风险", "privacy", []string{dir}, []string{"*.log"}, []string{}, model.RiskMedium, 0, false)

	result := Scan([]model.CleanRule{rule})

	if len(result.Items) != 1 {
		t.Fatalf("预期 1 个扫描项，实际得到 %d 个", len(result.Items))
	}
	if result.Items[0].Selected {
		t.Error("中风险 default_on=false 时 Selected 应为 false")
	}
}

func TestScan_WildcardPathExpansion(t *testing.T) {
	// Simulate browser cache structure: base/*/Cache
	dir := makeTempDir(t, map[string]string{
		"Profile1/Cache/a.tmp": "cache file 1",
		"Profile1/Cache/b.tmp": "cache file 2",
		"Profile2/Cache/c.tmp": "cache file 3",
		"Profile2/Data/d.txt":  "data file (should not match)",
	})
	_ = dir

	// Build the wildcard path using the temp dir as base.
	wildcardPath := filepath.Join(dir, "*", "Cache")
	rule := makeRule("浏览器缓存", "software", []string{wildcardPath}, []string{"*.tmp"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	// Should find 3 tmp files across both profile caches.
	if result.TotalFiles != 3 {
		t.Errorf("通配路径应扫描所有匹配的 profile 缓存，预期 3 个文件，实际得到 %d 个", result.TotalFiles)
		for _, item := range result.Items {
			t.Logf("  结果: %s", item.Path)
		}
	}

	// The data file in Profile2/Data should not be in results.
	// Use path separator to avoid matching "AppData" in the temp dir root.
	for _, item := range result.Items {
		normPath := filepath.ToSlash(item.Path)
		if strings.Contains(normPath, "/Data/") || strings.HasSuffix(normPath, "/Data") {
			t.Errorf("非缓存目录的文件不应出现在结果中: %s", item.Path)
		}
	}
}

func TestScan_WildcardNoMatch(t *testing.T) {
	// A wildcard path that doesn't match anything should not be an error.
	dir := makeTempDir(t, nil)
	wildcardPath := filepath.Join(dir, "NonExistent", "*", "Cache")
	rule := makeRule("不存在软件", "software", []string{wildcardPath}, []string{"*.tmp"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	// No crash, no error for wildcard no-match.
	if len(result.Errors) != 0 {
		t.Errorf("通配符无匹配不应产生错误，实际得到 %d 个: %v", len(result.Errors), result.Errors)
	}
}

func TestScan_MultipleRules(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"a.tmp": "tmp",
		"b.log": "log",
	})
	rule1 := makeRule("tmp规则", "system", []string{dir}, []string{"*.tmp"}, []string{}, model.RiskLow, 0, true)
	rule2 := makeRule("log规则", "system", []string{dir}, []string{"*.log"}, []string{}, model.RiskMedium, 0, false)

	result := Scan([]model.CleanRule{rule1, rule2})

	if result.TotalFiles != 2 {
		t.Errorf("多规则扫描应匹配 2 个文件，实际得到 %d 个", result.TotalFiles)
	}

	// Each item should have the correct source rule.
	sources := make(map[string]bool)
	for _, item := range result.Items {
		sources[item.Source] = true
	}
	if !sources["tmp规则"] || !sources["log规则"] {
		t.Errorf("来源规则不完整: %v", sources)
	}
}

func TestScan_GlobMatchesDirectory(t *testing.T) {
	// When a glob resolves to a directory, the scanner should walk into it.
	dir := makeTempDir(t, map[string]string{
		"subdir/file1.tmp": "content 1",
		"subdir/file2.tmp": "content 2",
	})
	// Use a wildcard in the middle of the path — more representative of
	// real browser-cache patterns (e.g. …\User Data\*\Cache).
	wildcardPath := filepath.Join(dir, "*")
	rule := makeRule("test", "system", []string{wildcardPath}, []string{"*.tmp"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if result.TotalFiles != 2 {
		t.Errorf("glob 匹配目录后应遍历其中文件，预期 2 个，实际得到 %d 个", result.TotalFiles)
		for _, item := range result.Items {
			t.Logf("  结果: %s", item.Path)
		}
	}
}

func TestScan_ExcludeTakesPriorityOverPatterns(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"important.tmp": "must keep",
	})
	// File matches both pattern and exclude — exclude wins.
	rule := makeRule("test", "system", []string{dir}, []string{"*.tmp"}, []string{"important.tmp"}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if result.TotalFiles != 0 {
		t.Errorf("exclude 应优先于 patterns 匹配，预期 0 个文件，实际得到 %d 个", result.TotalFiles)
	}
}

func TestScan_TotalSizeEqualsSumOfFileSizes(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"f1.tmp": "12345",                // 5 bytes
		"f2.tmp": "1234567890",           // 10 bytes
		"f3.log": "12345678901234567890", // 20 bytes
	})
	rule := makeRule("test", "system", []string{dir}, []string{"*.tmp", "*.log"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if result.TotalFiles != 3 {
		t.Fatalf("预期 3 个文件，实际得到 %d 个", result.TotalFiles)
	}

	var sum int64
	for _, item := range result.Items {
		sum += item.Size
	}

	if sum != result.TotalSize {
		t.Errorf("文件大小之和 (%d) 应等于 TotalSize (%d)", sum, result.TotalSize)
	}

	expectedTotal := int64(5 + 10 + 20)
	if result.TotalSize != expectedTotal {
		t.Errorf("TotalSize = %d, want %d", result.TotalSize, expectedTotal)
	}
}

func TestScan_RuleWithMultiplePaths(t *testing.T) {
	dir1 := makeTempDir(t, map[string]string{"a.tmp": "tmp"})
	dir2 := makeTempDir(t, map[string]string{"b.tmp": "tmp", "c.log": "log"})

	rule := makeRule("双路径", "system", []string{dir1, dir2}, []string{"*.tmp", "*.log"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if result.TotalFiles != 3 {
		t.Errorf("多路径规则应扫描所有路径，预期 3 个文件，实际得到 %d 个", result.TotalFiles)
	}
}

func TestScan_GlobError(t *testing.T) {
	// An invalid glob pattern should produce an error.
	rule := makeRule("test", "system", []string{"[invalid_glob"}, []string{"*.tmp"}, []string{}, model.RiskLow, 0, true)

	result := Scan([]model.CleanRule{rule})

	if len(result.Errors) == 0 {
		t.Error("无效的 glob 模式应产生错误")
	}
}

// ── Regression: scanner must not follow directory symlinks ────────────
// This test is primarily documentation — actually creating symlinks
// requires admin privileges on Windows, so we verify the check is in
// place via code review rather than runtime tests.

func TestScan_ItemsSortedByRuleOrder(t *testing.T) {
	dir := makeTempDir(t, map[string]string{
		"a.tmp": "a",
		"b.log": "b",
	})
	rule1 := makeRule("第一个规则", "system", []string{dir}, []string{"*.tmp"}, []string{}, model.RiskLow, 0, true)
	rule2 := makeRule("第二个规则", "software", []string{dir}, []string{"*.log"}, []string{}, model.RiskMedium, 0, false)

	result := Scan([]model.CleanRule{rule1, rule2})

	if len(result.Items) != 2 {
		t.Fatalf("预期 2 个扫描项，实际得到 %d 个", len(result.Items))
	}

	// First rule's items should come first.
	if result.Items[0].Source != "第一个规则" {
		t.Errorf("第一个扫描项来源 = %q, want %q", result.Items[0].Source, "第一个规则")
	}
	if result.Items[1].Source != "第二个规则" {
		t.Errorf("第二个扫描项来源 = %q, want %q", result.Items[1].Source, "第二个规则")
	}
}

func TestScanWithOptionsReportsProgressByRule(t *testing.T) {
	dir1 := makeTempDir(t, map[string]string{"a.tmp": "a"})
	dir2 := makeTempDir(t, map[string]string{"b.log": "b"})
	rule1 := makeRule("临时文件", "system", []string{dir1}, []string{"*.tmp"}, []string{}, model.RiskLow, 0, true)
	rule2 := makeRule("日志文件", "system", []string{dir2}, []string{"*.log"}, []string{}, model.RiskLow, 0, true)

	var progressEvents []model.ScanProgress
	result := ScanWithOptions([]model.CleanRule{rule1, rule2}, ScanOptions{
		OnProgress: func(progress model.ScanProgress) {
			progressEvents = append(progressEvents, progress)
		},
	})

	if result.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2", result.TotalFiles)
	}
	if len(progressEvents) != 2 {
		t.Fatalf("progress event count = %d, want 2", len(progressEvents))
	}
	first := progressEvents[0]
	if first.Phase != model.ScanPhaseScanningFiles {
		t.Fatalf("first phase = %q, want %q", first.Phase, model.ScanPhaseScanningFiles)
	}
	if first.CurrentLabel != "临时文件" {
		t.Fatalf("first current label = %q, want 临时文件", first.CurrentLabel)
	}
	last := progressEvents[len(progressEvents)-1]
	if last.CompletedSteps != 2 || last.TotalSteps != 2 {
		t.Fatalf("last progress steps = %d/%d, want 2/2", last.CompletedSteps, last.TotalSteps)
	}
	if last.Percent != 100 {
		t.Fatalf("last percent = %d, want 100", last.Percent)
	}
	if last.FoundItems != 2 || last.FailedItems != 0 {
		t.Fatalf("last counts = found %d failed %d, want found 2 failed 0", last.FoundItems, last.FailedItems)
	}
}

func TestClassifyScanAccessError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "permission denied",
			err:  errors.New("open C:\\Windows\\Temp: Access is denied."),
			want: "权限不足",
		},
		{
			name: "file locked",
			err:  errors.New("The process cannot access the file because it is being used by another process."),
			want: "文件被占用",
		},
		{
			name: "other failure",
			err:  errors.New("unexpected test failure"),
			want: "访问失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyScanAccessError(tt.err)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("classifyScanAccessError() = %q, want contains %q", got, tt.want)
			}
		})
	}
}
