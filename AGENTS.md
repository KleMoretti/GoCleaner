# AGENTS.md

## 项目背景

本项目为 GoCleaner：基于 Go 的 Windows 操作系统空间清理工具。项目来源于“操作系统空间清理工具的设计与实现”任务书，目标是在课程实训周期内完成一个可运行、可演示、可测试、可写报告的 Windows 空间清理工具。

核心功能范围：

```text
系统日志、缓存、临时文件清理
软件产生的可清理文件清理
隐私痕迹和无效注册表处理
不必要插件扫描或清理
垃圾文件粉碎
```

## 技术路线

默认采用：

```text
Go + Wails + React + TypeScript + JSON 规则文件
```

Go 负责扫描、清理、注册表访问、文件粉碎、日志记录等核心逻辑。React 负责桌面 UI 展示、勾选、确认、筛选和日志页面。

## 实现原则

所有后续开发必须遵循以下原则：

```text
安全优先
扫描先于清理
预览先于删除
高风险默认不选
注册表先备份再修改
失败必须可见
操作必须留痕
```

不得实现没有确认流程的一键深度清理。

## 风险控制规范

### 系统目录

涉及以下路径时必须标记为高风险，默认不勾选：

```text
C:\Windows\Temp
C:\Windows\Logs
C:\Windows\SoftwareDistribution\Download
```

如果清理失败，应记录权限不足、文件占用或其他具体错误，不得静默忽略。

### 浏览器数据

默认允许扫描或清理：

```text
Cache
Code Cache
GPUCache
ShaderCache
```

默认不得删除：

```text
History
Cookies
Login Data
Web Data
Preferences
```

如果未来支持这些敏感数据，必须放入高风险分类，并提供单独确认说明。

### QQ / 微信数据

不得清理聊天记录、图片、文件、数据库和账号相关文件。仅允许通过规则文件处理日志、缓存、临时文件。

### 注册表

注册表操作必须满足：

```text
仅扫描明确列出的安全路径
默认只展示问题项
删除前导出备份
删除前二次确认
删除失败必须展示原因
```

禁止实现全注册表扫描和全自动注册表修复。

### 文件粉碎

文件粉碎功能必须在 UI 或报告中说明限制：SSD、NTFS 日志、系统缓存、云同步目录等场景下无法保证专业取证级不可恢复。

## 推荐目录结构

```text
GoCleaner
├── frontend/
├── internal/
│   ├── app/
│   ├── scanner/
│   ├── cleaner/
│   ├── rules/
│   ├── windows/
│   ├── model/
│   └── logger/
├── configs/
├── data/
├── docs/
└── AGENTS.md
```

## Go 编码规范

使用 Go 标准工具链：

```text
gofmt
go test ./...
go vet ./...
```

约定：

```text
包名使用小写英文
错误必须返回或记录，不得吞掉
路径处理使用 filepath
环境变量展开集中封装
Windows API 调用集中放入 internal/windows
核心扫描逻辑必须可单元测试
```

不要在 UI 绑定层堆积业务逻辑。Wails 绑定层只负责接收参数、调用服务、返回结果。

## 前端规范

UI 应服务于清理工具的实际工作流，不做营销页。

必须具备：

```text
扫描入口
扫描结果表格
分类筛选
风险等级展示
勾选和全选控制
清理确认弹窗
清理结果反馈
操作日志页面
```

高风险项目必须有明显标识，并且默认不勾选。

## 规则文件规范

清理路径不得硬编码在扫描逻辑中，优先通过 JSON 规则配置。

规则字段建议：

```go
type CleanRule struct {
    Name       string   `json:"name"`
    Category   string   `json:"category"`
    Paths      []string `json:"paths"`
    Patterns   []string `json:"patterns"`
    Exclude    []string `json:"exclude"`
    Risk       string   `json:"risk"`
    MinAgeDays int      `json:"min_age_days"`
    DefaultOn  bool     `json:"default_on"`
}
```

规则加载时必须校验路径、风险等级和匹配模式。无效规则应记录错误并跳过，不应导致程序崩溃。

## 日志规范

操作日志至少记录：

```text
时间
操作类型
扫描文件数
清理文件数
释放空间
失败路径
失败原因
```

推荐使用 JSONL，便于后续在日志页面读取和展示。

## 测试规范

实现功能时优先用测试目录构造样例，不要直接拿真实系统目录做破坏性测试。

必须覆盖：

```text
规则加载
环境变量路径展开
文件匹配
大小统计
删除失败处理
文件粉碎流程
注册表备份逻辑
```

涉及真实系统路径、注册表和粉碎删除的测试，必须默认跳过或使用显式开关启用。

## 文档规范

实训文档统一放在 `docs/` 下。

推荐文档：

```text
docs/GoCleaner可行性计划.md
docs/系统分析文档.md
docs/系统设计文档.md
docs/实现与测试文档.md
docs/实训报告.md
```

文档应能解释每个高风险功能为什么这样设计，尤其是注册表、浏览器隐私数据、文件粉碎的限制。

## 交付标准

最终交付应满足：

```text
程序可以启动
可以完成扫描
可以展示扫描结果
可以清理低风险文件
可以记录操作日志
可以演示插件扫描
可以演示注册表无效项扫描和备份
可以演示文件粉碎
有测试记录
有完整报告
```

任何新功能都应优先保证可演示、可测试、可解释，而不是追求危险的深度清理能力。
