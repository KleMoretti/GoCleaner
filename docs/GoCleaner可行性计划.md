# GoCleaner 可行性计划

## 1. 项目定位

项目名称：GoCleaner

项目目标：实现一个面向 Windows 个人电脑的操作系统空间清理工具，覆盖任务书要求的系统垃圾清理、软件可清理文件清理、隐私痕迹和无效注册表处理、不必要插件清理、垃圾文件粉碎等功能。

本项目定位为课程实训可交付版本，不追求替代 360、火绒等商用清理软件，而是强调功能完整、风险可控、可演示、可测试、可写报告。

推荐技术路线：

```text
Go + Wails + React + JSON 规则文件 + Windows API
```

核心实现原则：

```text
先扫描，后预览
先分级，后确认
低风险默认可选
高风险默认不选
注册表先备份再修改
所有清理行为必须记录日志
```

## 2. 可行性结论

该计划整体可行，但必须控制范围。原计划中的注册表清理、浏览器隐私数据删除、系统目录深度清理、插件自动删除都属于高风险功能，不适合在 20 天实训周期内做成全自动深度清理。

更稳妥的实现方式是：

```text
文件清理：做完整扫描、预览、删除
隐私清理：只清缓存、Recent、JumpList、缩略图等安全范围
注册表清理：只做有限路径扫描、备份、手动确认删除
插件清理：默认只扫描展示，可选移动到隔离区
文件粉碎：实现教学意义上的覆写删除，并声明限制
```

这样既覆盖任务书要求，又能避免误删用户重要数据。

## 3. 功能范围

### 3.1 MVP 必做功能

| 模块 | 功能 | 实现策略 |
| --- | --- | --- |
| 系统垃圾扫描 | 扫描临时文件、日志、缓存 | 支持 `%TEMP%`、`%LOCALAPPDATA%\Temp`、`C:\Windows\Temp` |
| 软件缓存清理 | 清理浏览器、QQ、微信等缓存 | 通过 JSON 规则配置路径和风险等级 |
| 清理预览 | 展示路径、大小、类型、来源、风险 | UI 表格展示，支持勾选 |
| 安全删除 | 用户确认后删除 | 跳过锁定文件，记录失败原因 |
| 操作日志 | 记录扫描和清理行为 | 使用 JSONL 或文本日志 |
| 文件粉碎 | 覆写后删除指定文件 | 分块覆写、Sync、重命名、删除 |

### 3.2 增强功能

| 模块 | 功能 | 风险控制 |
| --- | --- | --- |
| 隐私痕迹清理 | Recent、JumpList、缩略图、浏览器缓存 | 不删除密码、Cookie、History 数据库 |
| 无效注册表扫描 | 检测无效启动项、无效路径引用 | 仅限安全路径，默认不勾选 |
| 插件扫描 | Chrome / Edge 扩展扫描 | 默认只展示，不直接删除 |
| 隔离区 | 删除前移动插件或高风险文件 | 支持手动恢复 |
| 规则管理 | 管理 JSON 清理规则 | 便于扩展和写报告 |

## 4. 风险边界

### 4.1 系统目录风险

`C:\Windows\Temp`、`C:\Windows\SoftwareDistribution\Download` 可能涉及管理员权限、系统服务占用和系统更新文件。实现时应将此类目录标记为高风险，默认不选中。

### 4.2 浏览器数据风险

浏览器的 `History`、`Cookies`、`Login Data` 等文件包含用户历史、登录状态、密码或账号相关数据。第一版不应默认删除这些文件。

推荐清理范围：

```text
Cache
Code Cache
GPUCache
ShaderCache
```

### 4.3 QQ / 微信数据风险

不得清理聊天记录、图片、文件、数据库。仅建议处理日志、缓存、临时目录，并通过规则文件明确标记。

### 4.4 注册表风险

注册表清理不得做一键全自动深度清理。推荐范围：

```text
HKCU\Software\Microsoft\Windows\CurrentVersion\Run
HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\RecentDocs
```

删除前必须备份，UI 中必须二次确认。

### 4.5 文件粉碎限制

文件粉碎在 SSD、NTFS 日志、系统缓存、云同步目录等场景下不能保证专业取证级不可恢复。报告中应明确说明本项目实现的是教学意义上的覆写粉碎。

## 5. 推荐架构

```text
GoCleaner
├── frontend/
│   └── Wails + React UI
├── internal/
│   ├── app/          Wails 绑定层
│   ├── scanner/      文件、隐私、注册表、插件扫描
│   ├── cleaner/      删除、隔离、粉碎
│   ├── rules/        JSON 规则加载和校验
│   ├── windows/      环境变量、权限、注册表封装
│   ├── model/        ScanItem、CleanResult 等结构体
│   └── logger/       JSONL 操作日志
├── configs/
│   └── cleaner_rules.json
├── data/
│   ├── operation.jsonl
│   ├── registry_backup/
│   └── quarantine/
├── docs/
│   ├── 系统分析文档.md
│   ├── 系统设计文档.md
│   ├── 实现与测试文档.md
│   └── 实训报告.md
└── AGENTS.md
```

## 6. 核心数据结构

### 6.1 清理规则

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

### 6.2 扫描项

```go
type ScanItem struct {
    ID           string `json:"id"`
    Path         string `json:"path"`
    Name         string `json:"name"`
    Type         string `json:"type"`
    Category     string `json:"category"`
    Size         int64  `json:"size"`
    Risk         string `json:"risk"`
    Source       string `json:"source"`
    LastModified int64  `json:"last_modified"`
    Selected     bool   `json:"selected"`
}
```

### 6.3 清理结果

```go
type CleanResult struct {
    DeletedFiles int      `json:"deleted_files"`
    FreedSize    int64    `json:"freed_size"`
    FailedFiles  []string `json:"failed_files"`
    Message      string   `json:"message"`
}
```

## 7. 主要流程

### 7.1 扫描流程

```text
用户点击开始扫描
↓
加载 cleaner_rules.json
↓
展开环境变量路径
↓
检查路径是否存在
↓
遍历目录并匹配规则
↓
计算大小、分类、风险等级
↓
返回扫描结果给 UI
```

### 7.2 清理流程

```text
用户勾选清理项
↓
程序重新校验文件是否存在
↓
根据风险等级生成确认提示
↓
用户确认
↓
执行删除或隔离
↓
记录成功和失败结果
↓
写入操作日志
```

### 7.3 注册表流程

```text
扫描指定注册表路径
↓
判断值中的路径是否失效
↓
标记为高风险
↓
展示给用户
↓
导出备份
↓
用户二次确认
↓
执行删除
```

### 7.4 文件粉碎流程

```text
用户选择文件
↓
检查文件存在和权限
↓
分块覆写内容
↓
Sync 刷盘
↓
关闭文件句柄
↓
随机重命名
↓
删除文件
↓
写入日志
```

## 8. 20 天实施计划

### 第 1-2 天：需求分析和风险边界

任务：

```text
阅读任务书
明确五类功能
确定安全边界
调研 Windows 常见垃圾路径
确定技术路线
```

产出：

```text
需求分析文档
功能清单
风险说明
技术选型说明
```

### 第 3-4 天：项目搭建和核心模型

任务：

```text
初始化 Go Module
搭建 Wails 项目
定义核心模型
实现 JSON 规则加载
实现环境变量路径展开
```

产出：

```text
可启动空项目
核心数据结构
规则文件初版
```

### 第 5-7 天：文件扫描 MVP

任务：

```text
实现目录遍历
实现文件匹配
实现大小统计
实现风险等级分类
完成系统临时文件和浏览器缓存扫描
```

产出：

```text
可运行扫描核心
扫描结果测试数据
基础单元测试
```

### 第 8-9 天：清理执行模块

任务：

```text
实现删除功能
实现锁定文件跳过
实现失败原因记录
实现操作日志
实现高风险确认逻辑
```

产出：

```text
可完成扫描到清理闭环
清理日志
异常处理记录
```

### 第 10-11 天：Wails 前端联调

任务：

```text
实现扫描按钮
实现结果表格
实现分类筛选
实现勾选清理
实现清理结果展示
```

产出：

```text
可演示桌面程序
核心功能截图
```

### 第 12-13 天：隐私痕迹和插件扫描

任务：

```text
实现 Recent 扫描
实现 JumpList 扫描
实现 Explorer 缩略图缓存扫描
实现 Chrome / Edge 插件 manifest 读取
```

产出：

```text
隐私痕迹扫描结果
插件扫描结果
```

### 第 14-15 天：注册表和文件粉碎

任务：

```text
实现 HKCU 启动项扫描
实现注册表备份
实现手动确认删除
实现文件覆写粉碎
```

产出：

```text
注册表扫描演示
文件粉碎演示
风险提示文案
```

### 第 16-17 天：体验完善和异常处理

任务：

```text
完善权限不足提示
增加扫描进度
增加清理确认弹窗
完善空结果和失败结果展示
整理日志页面
```

产出：

```text
较完整可演示版本
异常场景说明
```

### 第 18 天：测试文档

任务：

```text
编写文件扫描测试
编写删除失败测试
编写插件扫描测试
编写注册表备份测试
编写文件粉碎测试
```

产出：

```text
实现与测试文档
测试用例表
测试截图
```

### 第 19 天：报告和截图

任务：

```text
整理系统分析文档
整理系统设计文档
整理实训报告
补充架构图、流程图、核心代码说明和运行截图
```

产出：

```text
完整实训报告
答辩截图素材
```

### 第 20 天：打包和答辩准备

任务：

```text
打包 exe
准备 3-5 分钟演示流程
准备常见答辩问题
整理源码、报告和截图
```

产出：

```text
最终程序包
最终报告
答辩演示材料
```

## 9. 答辩亮点

```text
1. 采用 Go 实现系统工具核心逻辑。
2. 使用 Wails 将 Go 后端和 React 桌面界面结合。
3. 使用 JSON 规则文件扩展清理范围，便于维护。
4. 使用扫描预览、风险分级、用户确认机制降低误删风险。
5. 注册表清理采用备份和手动确认策略。
6. 文件粉碎模块体现底层文件 IO 能力。
7. 操作日志便于审计、测试和报告撰写。
```

## 10. 最终交付版本

```text
GoCleaner v1.0

核心功能：
1. 系统临时文件扫描与清理
2. 软件缓存扫描与清理
3. 隐私痕迹扫描与安全清理
4. 浏览器插件扫描
5. 有限注册表无效项扫描与备份清理
6. 文件粉碎
7. 清理规则配置
8. 操作日志
9. 扫描结果可视化展示
```

该版本能够覆盖任务书要求，同时将高风险操作控制在可解释、可演示、可测试的范围内，适合 20 天实训周期完成。
