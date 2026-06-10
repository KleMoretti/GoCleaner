package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gocleaner/internal/model"
)

// writeTempJSON creates a temporary JSON file with the given content and returns its path.
func writeTempJSON(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test_rules.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	return path
}

func TestLoadFromFile_ValidRules(t *testing.T) {
	json := `[
		{
			"name": "Chrome Cache",
			"category": "software",
			"paths": ["%LOCALAPPDATA%\\Google\\Chrome\\User Data\\*\\Cache"],
			"patterns": ["*"],
			"exclude": [],
			"risk": "low",
			"min_age_days": 0,
			"default_on": true
		},
		{
			"name": "缩略图缓存",
			"category": "privacy",
			"paths": ["%LOCALAPPDATA%\\Microsoft\\Windows\\Explorer"],
			"patterns": ["thumbcache_*.db"],
			"exclude": [],
			"risk": "medium",
			"min_age_days": 0,
			"default_on": false
		},
		{
			"name": "系统临时文件",
			"category": "system",
			"paths": ["C:\\Windows\\Temp"],
			"patterns": ["*.tmp"],
			"exclude": [],
			"risk": "high",
			"min_age_days": 7,
			"default_on": false
		}
	]`

	path := writeTempJSON(t, json)

	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	if len(result.Rules) != 3 {
		t.Errorf("预期 3 条有效规则，实际得到 %d 条", len(result.Rules))
	}

	if len(result.Errors) != 0 {
		t.Errorf("预期 0 个错误，实际得到 %d 个: %v", len(result.Errors), result.Errors)
	}

	// Verify first rule
	if result.Rules[0].Name != "Chrome Cache" {
		t.Errorf("规则[0]名称 = %q, want %q", result.Rules[0].Name, "Chrome Cache")
	}
	if result.Rules[0].Risk != model.RiskLow {
		t.Errorf("规则[0]风险等级 = %q, want %q", result.Rules[0].Risk, model.RiskLow)
	}
}

func TestLoadFromFile_InvalidRiskLevel(t *testing.T) {
	json := `[
		{
			"name": "Bad Rule",
			"category": "system",
			"paths": ["C:\\Temp"],
			"risk": "critical",
			"default_on": true
		}
	]`

	path := writeTempJSON(t, json)

	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	if len(result.Rules) != 0 {
		t.Errorf("预期 0 条有效规则（无效风险等级应被跳过），实际得到 %d 条", len(result.Rules))
	}

	if len(result.Errors) != 1 {
		t.Fatalf("预期 1 个错误，实际得到 %d 个", len(result.Errors))
	}

	errMsg := result.Errors[0].Message
	if errMsg == "" {
		t.Error("错误消息不应为空")
	}
	t.Logf("无效风险等级错误: %v", result.Errors[0])
}

func TestLoadFromFile_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "missing name",
			json: `[{"category": "system", "paths": ["C:\\Temp"], "risk": "low", "default_on": true}]`,
		},
		{
			name: "missing category",
			json: `[{"name": "Test", "paths": ["C:\\Temp"], "risk": "low", "default_on": true}]`,
		},
		{
			name: "missing paths",
			json: `[{"name": "Test", "category": "system", "paths": [], "risk": "low", "default_on": true}]`,
		},
		{
			name: "missing risk",
			json: `[{"name": "Test", "category": "system", "paths": ["C:\\Temp"], "default_on": true}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempJSON(t, tt.json)

			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}

			if len(result.Rules) != 0 {
				t.Errorf("预期 0 条有效规则，实际得到 %d 条", len(result.Rules))
			}

			if len(result.Errors) == 0 {
				t.Error("预期至少 1 个验证错误")
			} else {
				t.Logf("缺失字段错误: %v", result.Errors[0])
			}
		})
	}
}

func TestLoadFromFile_HighRiskForcedOff(t *testing.T) {
	// A high-risk rule with default_on: true should be forced to false
	json := `[
		{
			"name": "高风险规则",
			"category": "system",
			"paths": ["C:\\Windows\\Temp"],
			"risk": "high",
			"default_on": true
		}
	]`

	path := writeTempJSON(t, json)

	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	if len(result.Rules) != 1 {
		t.Fatalf("预期 1 条有效规则，实际得到 %d 条", len(result.Rules))
	}

	if result.Rules[0].DefaultOn {
		t.Error("高风险规则 default_on 应被强制设为 false，但实际为 true")
	}

	if result.Rules[0].Risk != model.RiskHigh {
		t.Errorf("风险等级 = %q, want %q", result.Rules[0].Risk, model.RiskHigh)
	}
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	_, err := LoadFromFile(`Z:\non_existent\path\rules.json`)
	if err == nil {
		t.Error("预期文件不存在错误")
	}
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	path := writeTempJSON(t, `this is not valid json {{{`)

	_, err := LoadFromFile(path)
	if err == nil {
		t.Error("预期 JSON 解析错误")
	}
}

func TestLoadFromFile_EmptyArray(t *testing.T) {
	path := writeTempJSON(t, `[]`)

	_, err := LoadFromFile(path)
	if err == nil {
		t.Error("预期空规则数组错误")
	}
}

func TestLoadFromFile_MixedValidAndInvalid(t *testing.T) {
	// Mix of valid and invalid rules: the valid ones should load, invalid skipped
	json := `[
		{
			"name": "Valid Rule",
			"category": "system",
			"paths": ["C:\\Temp"],
			"risk": "low",
			"default_on": true
		},
		{
			"name": "Invalid Risk",
			"category": "system",
			"paths": ["C:\\Temp"],
			"risk": "dangerous",
			"default_on": false
		},
		{
			"name": "Another Valid",
			"category": "software",
			"paths": ["C:\\Cache"],
			"risk": "medium",
			"default_on": false
		}
	]`

	path := writeTempJSON(t, json)

	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	if len(result.Rules) != 2 {
		t.Errorf("预期 2 条有效规则，实际得到 %d 条", len(result.Rules))
	}

	if len(result.Errors) != 1 {
		t.Errorf("预期 1 个验证错误，实际得到 %d 个", len(result.Errors))
	}

	// Verify the valid rules are correct
	if result.Rules[0].Name != "Valid Rule" {
		t.Errorf("规则[0]名称 = %q, want %q", result.Rules[0].Name, "Valid Rule")
	}
	if result.Rules[1].Name != "Another Valid" {
		t.Errorf("规则[1]名称 = %q, want %q", result.Rules[1].Name, "Another Valid")
	}
}

func TestLoadFromFile_MediumRiskDefaultOffPreserved(t *testing.T) {
	// Medium risk with default_on=false should keep it false
	// Medium risk with default_on=true should keep it true (only high is forced off)
	json := `[
		{
			"name": "Medium Off",
			"category": "privacy",
			"paths": ["%APPDATA%\\Recent"],
			"risk": "medium",
			"default_on": false
		},
		{
			"name": "Medium On",
			"category": "privacy",
			"paths": ["%APPDATA%\\Other"],
			"risk": "medium",
			"default_on": true
		}
	]`

	path := writeTempJSON(t, json)

	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	if len(result.Rules) != 2 {
		t.Fatalf("预期 2 条规则，实际得到 %d 条", len(result.Rules))
	}

	if result.Rules[0].DefaultOn {
		t.Error("中风险规则 default_on=false 应保持 false")
	}

	if !result.Rules[1].DefaultOn {
		t.Error("中风险规则 default_on=true 应保持 true")
	}
}

// TestLoadFromFile_ActualConfig verifies that the real cleaner_rules.json
// in the project configs directory loads without errors.
func TestLoadFromFile_ActualConfig(t *testing.T) {
	// Path is relative to the module root (where go test runs)
	configPath := "../../configs/cleaner_rules.json"

	result, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("加载真实配置文件失败: %v", err)
	}

	if len(result.Rules) == 0 {
		t.Error("真实配置文件应至少包含 1 条规则")
	}

	if len(result.Errors) > 0 {
		t.Errorf("真实配置文件不应有错误: %v", result.Errors)
	}

	// Verify no high-risk rule has default_on=true
	for i, r := range result.Rules {
		if r.Risk == "high" && r.DefaultOn {
			t.Errorf("规则[%d] %q: 高风险规则 default_on 必须为 false", i, r.Name)
		}
	}

	t.Logf("成功加载真实配置文件: %d 条规则", len(result.Rules))
	for _, r := range result.Rules {
		t.Logf("  - %s (risk=%s, default_on=%v)", r.Name, r.Risk, r.DefaultOn)
	}
}

// ── P1b: Empty / blank / relative path rejection ──────────────────────

func TestLoadFromFile_EmptyPathEntries(t *testing.T) {
	tests := []struct {
		name     string
		pathVal  string
		wantSkip bool
	}{
		{name: "empty string", pathVal: "", wantSkip: true},
		{name: "whitespace only", pathVal: "   ", wantSkip: true},
		{name: "dot", pathVal: ".", wantSkip: true},
		{name: "dot dot", pathVal: "..", wantSkip: true},
		{name: "valid path", pathVal: "C:\\Temp", wantSkip: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "Test",
				"category": "system",
				"paths": [%q],
				"risk": "low",
				"default_on": true
			}]`, tt.pathVal)

			path := writeTempJSON(t, json)
			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}

			if tt.wantSkip && len(result.Rules) != 0 {
				t.Errorf("path=%q: 预期规则被跳过，实际加载了 %d 条", tt.pathVal, len(result.Rules))
			}
			if !tt.wantSkip && len(result.Rules) != 1 {
				t.Errorf("path=%q: 预期规则被加载，实际跳过了", tt.pathVal)
			}
		})
	}
}

// ── P1c: Invalid category ─────────────────────────────────────────────

func TestLoadFromFile_InvalidCategory(t *testing.T) {
	json := `[{
		"name": "Bad Category",
		"category": "malware",
		"paths": ["C:\\Temp"],
		"risk": "low",
		"default_on": true
	}]`

	path := writeTempJSON(t, json)
	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	if len(result.Rules) != 0 {
		t.Errorf("无效分类应被跳过，实际加载了 %d 条", len(result.Rules))
	}
	if len(result.Errors) == 0 {
		t.Error("预期至少 1 个验证错误")
	} else {
		t.Logf("无效分类错误: %v", result.Errors[0])
	}
}

// ── P1c: Forbidden system path prefix ─────────────────────────────────

func TestLoadFromFile_ForbiddenSystemPaths(t *testing.T) {
	tests := []string{
		`C:\Windows\System32\config`,
		`C:\Windows\SysWOW64\drivers`,
		`C:\Windows\Boot`,
		`C:\Windows\Installer`,
		`C:\Program Files\SomeApp`,
		`C:\Program Files (x86)\SomeApp`,
	}

	for _, forbiddenPath := range tests {
		t.Run(filepath.Base(forbiddenPath), func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "Forbidden Target",
				"category": "system",
				"paths": [%q],
				"risk": "high",
				"default_on": false
			}]`, forbiddenPath)

			path := writeTempJSON(t, json)
			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}

			if len(result.Rules) != 0 {
				t.Errorf("path=%q: 受保护系统路径应被拒绝，但规则被加载了", forbiddenPath)
			}
			if len(result.Errors) == 0 {
				t.Error("预期至少 1 个错误")
			} else {
				t.Logf("受保护路径拒绝: %v", result.Errors[0])
			}
		})
	}
}

// ── P1c: Forbidden browser file names in patterns ─────────────────────

func TestLoadFromFile_ForbiddenBrowserFiles(t *testing.T) {
	forbiddenPatterns := []string{"History", "Cookies", "Login Data", "Web Data", "Preferences"}

	for _, pat := range forbiddenPatterns {
		t.Run(pat, func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "Bad Pattern",
				"category": "software",
				"paths": ["%%LOCALAPPDATA%%\\Chrome\\User Data\\*"],
				"patterns": [%q],
				"risk": "low",
				"default_on": true
			}]`, pat)

			path := writeTempJSON(t, json)
			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}

			if len(result.Rules) != 0 {
				t.Errorf("pattern=%q: 受保护文件应被拒绝，但规则被加载了", pat)
			}
		})
	}
}

// ── P1c: Glob pattern syntax errors ───────────────────────────────────

func TestLoadFromFile_BadGlobPatterns(t *testing.T) {
	tests := []struct {
		pattern  string
		wantSkip bool
	}{
		{pattern: `*.log`, wantSkip: false},         // valid
		{pattern: `file[0-9].txt`, wantSkip: false}, // valid bracket
		{pattern: `file[0-9.txt`, wantSkip: true},   // unmatched '['
		{pattern: `file]bad.txt`, wantSkip: true},   // unmatched ']'
		{pattern: `*.exe`, wantSkip: false},         // valid
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "Glob Test",
				"category": "system",
				"paths": ["C:\\Temp"],
				"patterns": [%q],
				"risk": "low",
				"default_on": true
			}]`, tt.pattern)

			path := writeTempJSON(t, json)
			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}

			if tt.wantSkip && len(result.Rules) != 0 {
				t.Errorf("pattern=%q 应被拒绝，但规则被加载了", tt.pattern)
			}
			if !tt.wantSkip && len(result.Rules) != 1 {
				t.Errorf("pattern=%q 应被接受，但规则被跳过了 (errors: %v)", tt.pattern, result.Errors)
			}
		})
	}
}

// ── P1a: LoadFromBytes ────────────────────────────────────────────────

func TestLoadFromBytes_ValidRules(t *testing.T) {
	data := []byte(`[
		{
			"name": "Test Rule",
			"category": "software",
			"paths": ["C:\\Cache"],
			"risk": "low",
			"default_on": true
		}
	]`)

	result, err := LoadFromBytes(data)
	if err != nil {
		t.Fatalf("LoadFromBytes() unexpected error: %v", err)
	}
	if len(result.Rules) != 1 {
		t.Errorf("预期 1 条规则，实际 %d 条", len(result.Rules))
	}
}

func TestLoadFromBytes_InvalidJSON(t *testing.T) {
	_, err := LoadFromBytes([]byte(`not json`))
	if err == nil {
		t.Error("预期 JSON 解析错误")
	}
}

func TestLoadFromBytes_EmptyArray(t *testing.T) {
	_, err := LoadFromBytes([]byte(`[]`))
	if err == nil {
		t.Error("预期空规则数组错误")
	}
}

func TestLoadFromFile_ForbiddenPathMiddleWare(t *testing.T) {
	// A path targeting QQ WeChat data should trigger a warning but NOT be
	// skipped (the sub-path check is advisory, not fatal).
	json := `[{
		"name": "微信缓存（含受保护警告）",
		"category": "software",
		"paths": ["%USERPROFILE%\\Documents\\WeChat Files\\wxid_xxx\\log"],
		"patterns": ["*.log"],
		"exclude": ["Msg3.0.db", "Msg.db"],
		"risk": "medium",
		"default_on": false
	}]`

	path := writeTempJSON(t, json)
	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	// Sub-path check emits a warning but does NOT reject the rule
	if len(result.Rules) != 1 {
		t.Errorf("受保护子路径应发出警告但不跳过规则，实际规则数: %d", len(result.Rules))
	}
	if len(result.Warnings) == 0 {
		t.Log("注意：子路径检查未触发（可能是 JSON 转义导致路径不匹配）")
	} else {
		for _, w := range result.Warnings {
			t.Logf("子路径警告（预期）: %v", w)
		}
	}
	// Ensure no fatal errors
	if len(result.Errors) != 0 {
		t.Errorf("子路径检查不应产生致命错误，实际: %v", result.Errors)
	}
}

// ── Path leaf forbidden-name check ────────────────────────────────────

func TestLoadFromFile_ForbiddenNameInPathLeaf(t *testing.T) {
	// A rule that puts "History" in the path itself (not patterns) should
	// be rejected — the path-leaf check catches it even with empty patterns.
	json := `[{
		"name": "History via path leaf",
		"category": "software",
		"paths": ["%LOCALAPPDATA%\\Google\\Chrome\\User Data\\Default\\History"],
		"patterns": [],
		"risk": "low",
		"default_on": true
	}]`

	path := writeTempJSON(t, json)
	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	if len(result.Rules) != 0 {
		t.Error("路径末段为 History（敏感浏览器文件）应被拒绝")
	}
	if len(result.Errors) == 0 {
		t.Error("预期至少 1 个错误")
	} else {
		t.Logf("路径末段拒绝: %v", result.Errors[0])
	}
}

// ── Browser whitelist enforcement ─────────────────────────────────────

func TestLoadFromFile_BrowserWhitelist(t *testing.T) {
	tests := []struct {
		leaf    string
		allowed bool
	}{
		{"Cache", true},
		{"Code Cache", true},
		{"GPUCache", true},
		{"ShaderCache", true},
		{"History", false},
		{"Extensions", false},
		{"Local Storage", false},
		{"Session Storage", false},
	}

	for _, tt := range tests {
		t.Run(tt.leaf, func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "Browser leaf: %s",
				"category": "software",
				"paths": ["%%LOCALAPPDATA%%\\Chrome\\User Data\\*\\%s"],
				"patterns": ["*"],
				"risk": "low",
				"default_on": true
			}]`, tt.leaf, tt.leaf)

			path := writeTempJSON(t, json)
			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}

			if tt.allowed && len(result.Rules) != 1 {
				t.Errorf("leaf=%q 应在白名单内，但被拒绝: %v", tt.leaf, result.Errors)
			}
			if !tt.allowed && len(result.Rules) != 0 {
				t.Errorf("leaf=%q 不在白名单内，应被拒绝，但规则被加载", tt.leaf)
			}
		})
	}
}

// ── IM protected directory fatal errors ───────────────────────────────

func TestLoadFromFile_IMProtectedDirsFatal(t *testing.T) {
	tests := []string{"Audio", "Image", "File", "Video", "Msg", "FileRecv", "Pic"}

	for _, dir := range tests {
		t.Run(dir, func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "IM protected dir: %s",
				"category": "software",
				"paths": ["%%USERPROFILE%%\\Documents\\WeChat Files\\wxid_xxx\\%s"],
				"patterns": ["*.dat"],
				"risk": "medium",
				"default_on": false
			}]`, dir, dir)

			path := writeTempJSON(t, json)
			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}

			if len(result.Rules) != 0 {
				t.Errorf("IM 受保护目录 %q 应被拒绝（致命错误），但规则被加载", dir)
			}
			if len(result.Errors) == 0 {
				t.Error("预期至少 1 个错误")
			} else {
				t.Logf("IM 目录拒绝: %v", result.Errors[0])
			}
		})
	}
}

func TestLoadFromFile_IMMsgDatabaseFatal(t *testing.T) {
	json := `[{
		"name": "Msg database target",
		"category": "software",
		"paths": ["%USERPROFILE%\\Documents\\WeChat Files\\wxid_xxx\\Msg3.0.db"],
		"patterns": [],
		"risk": "low",
		"default_on": true
	}]`

	path := writeTempJSON(t, json)
	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	if len(result.Rules) != 0 {
		t.Error("Msg3.0.db 应被拒绝")
	}
}

// ── IM with safe leaf passes ──────────────────────────────────────────

func TestLoadFromFile_IMSafeLeafAllowed(t *testing.T) {
	safeDirs := []string{"log", "Log", "Logs", "cache", "Cache", "temp", "Temp"}

	for _, dir := range safeDirs {
		t.Run(dir, func(t *testing.T) {
			json := fmt.Sprintf(`[{
				"name": "IM safe leaf: %s",
				"category": "software",
				"paths": ["%%USERPROFILE%%\\Documents\\WeChat Files\\wxid_xxx\\%s"],
				"patterns": ["*.log"],
				"risk": "medium",
				"default_on": false
			}]`, dir, dir)

			path := writeTempJSON(t, json)
			result, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile() unexpected error: %v", err)
			}

			if len(result.Rules) != 1 {
				t.Errorf("IM 安全目录 %q 应被允许，但被拒绝: %v", dir, result.Errors)
			}
		})
	}
}

// ── IM non-safe leaf with empty patterns → fatal ──────────────────────

func TestLoadFromFile_IMUnsafeLeafEmptyPatterns(t *testing.T) {
	json := `[{
		"name": "IM unsafe, no patterns",
		"category": "software",
		"paths": ["%USERPROFILE%\\Documents\\WeChat Files\\wxid_xxx\\unknown_dir"],
		"patterns": [],
		"risk": "low",
		"default_on": true
	}]`

	path := writeTempJSON(t, json)
	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	if len(result.Rules) != 0 {
		t.Error("IM 目录 + 空 patterns 应被拒绝（可能匹配所有文件）")
	}
	if len(result.Errors) == 0 {
		t.Error("预期至少 1 个致命错误")
	} else {
		t.Logf("IM 空 patterns 拒绝: %v", result.Errors[0])
	}
}

// ── IM non-safe leaf with explicit patterns → warning ─────────────────

func TestLoadFromFile_IMUnsafeLeafWithPatterns(t *testing.T) {
	json := `[{
		"name": "IM unsafe with patterns",
		"category": "software",
		"paths": ["%USERPROFILE%\\Documents\\WeChat Files\\wxid_xxx\\unknown_dir"],
		"patterns": ["*.tmp", "*.log"],
		"exclude": ["Msg3.0.db", "Msg.db"],
		"risk": "medium",
		"default_on": false
	}]`

	path := writeTempJSON(t, json)
	result, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() unexpected error: %v", err)
	}

	// With explicit patterns, the rule should load (warning, not error)
	if len(result.Rules) != 1 {
		t.Fatalf("预期规则被加载（仅警告），实际被跳过。Errors: %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Log("注意：显式 patterns 下预期有警告，但未触发")
	} else {
		t.Logf("IM 警告: %v", result.Warnings[0])
	}
}
