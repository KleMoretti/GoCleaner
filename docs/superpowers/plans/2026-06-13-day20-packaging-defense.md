# Day 20 Packaging And Defense Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 GoCleaner 第 20 天交付准备：通过 GitHub 自动打包发布 Windows exe，准备 3-5 分钟演示流程，并整理安全性、注册表、文件粉碎、规则扩展相关答辩问题。

**Architecture:** 保持本地代码、测试、文档和发布流程分离。GitHub Actions 负责在 tag 或手动触发时执行测试、构建 Wails Windows 程序、生成 exe 并上传到 GitHub Release；答辩材料围绕“扫描先于清理、预览先于删除、高风险默认不选、失败可见、操作留痕”组织。

**Tech Stack:** Go 1.24.13, Wails v2.12.0, React 18, TypeScript, Vite, GitHub Actions, GitHub Releases.

---

## 文件职责

- `.github/workflows/release.yml`：新增正式发布 workflow，负责测试、Wails 打包、创建或更新 GitHub Release，并上传 `GoCleaner-<version>-windows-amd64.exe`。
- `.github/workflows/ci.yml`：保留现有 CI。它已经负责 push / pull_request 的测试，以及手动或 release 事件下上传构建 artifact；第 20 天不强制修改，避免把 CI 和 Release 权限混在一起。
- `docs/superpowers/plans/2026-06-13-day20-packaging-defense.md`：第 20 天执行计划。
- `docs/实现与测试文档.md`：发布完成后补充最终测试命令、GitHub Release 截图和 exe 下载说明。
- `docs/实训报告.md`：发布完成后补充最终交付版本、演示流程、风险控制和答辩口径。

## 第 20 天时间安排

| 时间 | 任务 | 产出 |
| --- | --- | --- |
| 09:00-09:30 | 冻结功能范围，确认不再加入高风险深度清理能力 | 最终功能清单 |
| 09:30-11:00 | 配置 GitHub 自动打包发布流程 | `.github/workflows/release.yml` |
| 11:00-12:00 | 本地执行测试和构建检查 | `go test ./...`、`go vet ./...`、`npm run test:summary`、`npm run build` 结果 |
| 13:30-14:30 | 推送 `v1.0.0` tag 或手动触发 workflow，验证 Release 附件 | GitHub Release 页面与 exe 附件 |
| 14:30-15:30 | 准备 3-5 分钟演示流程 | 演示脚本和操作顺序 |
| 15:30-16:30 | 准备答辩问题 | 安全性、注册表、文件粉碎、规则扩展问答 |
| 16:30-17:30 | 更新报告和截图 | 最终文档、截图、发布链接 |
| 17:30-18:00 | 彩排一次完整演示 | 计时在 3-5 分钟内 |

---

### Task 1: 新增 GitHub 自动打包发布 Workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: 创建 release workflow**

创建 `.github/workflows/release.yml`，内容如下：

```yaml
name: Release

on:
  push:
    tags:
      - "v*"
  workflow_dispatch:
    inputs:
      version:
        description: "Release version, for example v1.0.0"
        required: true
        type: string
      prerelease:
        description: "Mark this release as a prerelease"
        required: true
        default: false
        type: boolean

permissions:
  contents: write

concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false

jobs:
  build-windows:
    name: Build and publish Windows exe
    runs-on: windows-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Install frontend dependencies
        working-directory: frontend
        run: npm ci

      - name: Test frontend summary logic
        working-directory: frontend
        run: npm run test:summary

      - name: Build frontend
        working-directory: frontend
        run: npm run build

      - name: Run Go tests
        run: go test ./...

      - name: Run Go vet
        run: go vet ./...

      - name: Install Wails CLI
        run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

      - name: Build Windows package
        run: wails build -clean -platform windows/amd64

      - name: Resolve release metadata
        id: meta
        shell: pwsh
        run: |
          if ("${{ github.event_name }}" -eq "workflow_dispatch") {
            $version = "${{ inputs.version }}"
          } else {
            $version = "${{ github.ref_name }}"
          }

          if (-not $version.StartsWith("v")) {
            throw "Release version must start with v, for example v1.0.0"
          }

          $assetName = "GoCleaner-$version-windows-amd64.exe"
          Copy-Item -LiteralPath "build/bin/GoCleaner.exe" -Destination $assetName

          "version=$version" >> $env:GITHUB_OUTPUT
          "asset_name=$assetName" >> $env:GITHUB_OUTPUT

      - name: Upload workflow artifact
        uses: actions/upload-artifact@v4
        with:
          name: ${{ steps.meta.outputs.asset_name }}
          path: ${{ steps.meta.outputs.asset_name }}
          if-no-files-found: error

      - name: Publish GitHub Release
        shell: pwsh
        env:
          GH_TOKEN: ${{ github.token }}
          VERSION: ${{ steps.meta.outputs.version }}
          ASSET_NAME: ${{ steps.meta.outputs.asset_name }}
          PRERELEASE: ${{ inputs.prerelease || false }}
        run: |
          $releaseExists = gh release view $env:VERSION 2>$null

          if ($LASTEXITCODE -eq 0) {
            gh release upload $env:VERSION $env:ASSET_NAME --clobber
            exit $LASTEXITCODE
          }

          $args = @(
            "release", "create", $env:VERSION,
            $env:ASSET_NAME,
            "--title", "GoCleaner $env:VERSION",
            "--notes", "Windows amd64 executable package for GoCleaner.",
            "--target", $env:GITHUB_SHA
          )

          if ($env:PRERELEASE -eq "true") {
            $args += "--prerelease"
          }

          gh @args
```

- [ ] **Step 2: 提交前检查 workflow 文件**

运行：

```powershell
git diff -- .github/workflows/release.yml
```

预期：只看到新增 `Release` workflow，不应修改现有业务代码。

- [ ] **Step 3: 本地执行后端验证**

运行：

```powershell
go test ./...
go vet ./...
```

预期：所有 Go 测试通过，`go vet` 无诊断。真实注册表写入测试默认跳过，不应为了答辩开启破坏性测试。

- [ ] **Step 4: 本地执行前端验证**

运行：

```powershell
cd frontend
npm run test:summary
npm run build
cd ..
```

预期：TypeScript 汇总逻辑测试通过，Vite 生产构建成功。

- [ ] **Step 5: 触发 GitHub 自动发布**

推荐正式交付使用 tag 触发：

```powershell
git tag v1.0.0
git push origin v1.0.0
```

备用方案是在 GitHub 页面进入 `Actions -> Release -> Run workflow`，输入：

```text
version: v1.0.0
prerelease: false
```

预期：GitHub Actions 完成 `Build and publish Windows exe`，Release 页面出现 `GoCleaner-v1.0.0-windows-amd64.exe`。

---

### Task 2: 最终打包验收

**Files:**
- Modify: `docs/实现与测试文档.md`
- Modify: `docs/实训报告.md`

- [ ] **Step 1: 下载 Release exe 并运行冒烟测试**

从 GitHub Release 下载：

```text
GoCleaner-v1.0.0-windows-amd64.exe
```

验收动作：

```text
1. 程序能启动。
2. 点击开始扫描后能展示扫描结果。
3. 高风险项目默认不勾选。
4. 低风险临时文件能在确认后清理。
5. 清理失败时能看到失败原因。
6. 操作日志页面能看到扫描或清理记录。
7. 插件扫描、注册表扫描、文件粉碎入口可演示。
```

- [ ] **Step 2: 记录最终测试命令**

在 `docs/实现与测试文档.md` 中补充：

```markdown
## 第 20 天最终打包验证

| 验证项 | 命令或操作 | 通过标准 |
| --- | --- | --- |
| Go 单元测试 | `go test ./...` | 全部测试通过 |
| Go 静态检查 | `go vet ./...` | 无 vet 诊断 |
| 前端汇总逻辑测试 | `npm run test:summary` | TypeScript 编译和断言通过 |
| 前端生产构建 | `npm run build` | Vite 构建成功 |
| GitHub Release 打包 | 推送 `v1.0.0` tag 或手动运行 Release workflow | Release 附件生成 `GoCleaner-v1.0.0-windows-amd64.exe` |
| exe 冒烟测试 | 下载 Release exe 后启动 | 能扫描、预览、确认清理、展示失败、记录日志 |
```

- [ ] **Step 3: 记录最终交付版本**

在 `docs/实训报告.md` 中补充：

```markdown
## 最终交付版本

本项目最终交付版本为 `GoCleaner v1.0.0`。程序通过 GitHub Actions 自动完成测试、Wails Windows 打包和 GitHub Release 发布，发布附件为 `GoCleaner-v1.0.0-windows-amd64.exe`。

该版本保留安全边界：扫描先于清理，预览先于删除，高风险项目默认不勾选，注册表修改前导出备份，文件粉碎仅处理用户手动选择的普通文件，所有失败原因和操作结果写入日志。
```

---

### Task 3: 3-5 分钟演示流程

**Files:**
- Modify: `docs/实训报告.md`

- [ ] **Step 1: 准备演示数据**

演示前准备：

```text
1. 使用临时目录或测试文件，不使用真实重要文件。
2. 准备一个可删除的低风险 `.tmp` 或 `.log` 文件。
3. 准备一个普通文本文件用于文件粉碎演示。
4. 注册表演示优先使用扫描展示和备份说明，不删除真实业务软件启动项。
5. 打开 GitHub Release 页面，确认 exe 附件可见。
```

- [ ] **Step 2: 按 3-5 分钟脚本演示**

演示顺序：

```text
0:00-0:30 说明项目定位：GoCleaner 是 Windows 空间清理工具，重点是安全可控、可演示、可测试。
0:30-1:20 展示主界面和扫描：点击开始扫描，说明规则文件、分类、大小统计和风险等级。
1:20-2:00 展示预览和清理确认：筛选低风险项目，勾选后打开确认弹窗，强调高风险默认不选。
2:00-2:40 展示失败可见和操作日志：说明权限不足、文件占用等失败不会静默忽略，会展示并写入日志。
2:40-3:20 展示插件扫描和注册表扫描：说明插件默认展示，注册表只扫描 HKCU 安全范围，删除前备份并二次确认。
3:20-4:00 展示文件粉碎：选择测试文件，说明 1/3/7 次覆写、确认流程和 SSD/NTFS/云同步限制。
4:00-4:40 展示 GitHub 自动发布：打开 Release 页面，说明 tag 触发自动测试、打包、发布 exe。
4:40-5:00 总结：项目覆盖任务书功能，同时通过风险分级、规则配置、日志留痕保证可解释。
```

- [ ] **Step 3: 演示时避免的操作**

答辩现场不做：

```text
1. 不扫描全注册表。
2. 不删除 Cookies、History、Login Data、Web Data、Preferences。
3. 不删除 QQ/微信聊天记录、图片、文件、数据库和账号相关文件。
4. 不对真实重要文件做粉碎。
5. 不启用无确认流程的一键深度清理。
```

---

### Task 4: 答辩问题准备

**Files:**
- Modify: `docs/实训报告.md`

- [ ] **Step 1: 安全性问题**

答辩问答：

```text
问：为什么不做一键深度清理？
答：清理工具的主要风险是误删和不可恢复。本项目采用扫描先于清理、预览先于删除、风险分级、用户确认和操作日志。高风险项目默认不勾选，因此不实现无确认流程的一键深度清理。

问：系统目录为什么标高风险？
答：例如 C:\Windows\Temp、C:\Windows\Logs、C:\Windows\SoftwareDistribution\Download 可能涉及管理员权限、系统服务占用或 Windows 更新文件。程序可以扫描展示，但默认不勾选，清理失败会展示权限不足或文件占用等原因。

问：如何避免误删浏览器隐私数据？
答：默认只允许处理 Cache、Code Cache、GPUCache、ShaderCache 这类缓存目录，不删除 History、Cookies、Login Data、Web Data、Preferences。如果未来支持敏感数据，也必须单独归入高风险分类并二次确认。
```

- [ ] **Step 2: 注册表问题**

答辩问答：

```text
问：为什么不做全注册表扫描？
答：全注册表扫描容易误判，课程项目难以验证所有软件和系统项的安全性。本项目只扫描明确列出的安全路径，例如 HKCU Run 启动项，避免扩大破坏范围。

问：注册表删除前如何保证可恢复？
答：删除前会先导出 .reg 备份，备份失败则拒绝删除。删除时只处理用户勾选的 value，不删除整个 key，并且执行前需要二次确认。

问：无效启动项如何判断？
答：程序读取启动项命令，解析引号、参数和环境变量后定位目标路径。如果目标路径不存在，则展示为无效项；如果命令无法明确解析，不会直接判定为可删除。
```

- [ ] **Step 3: 文件粉碎问题**

答辩问答：

```text
问：文件粉碎是否能保证完全不可恢复？
答：不能保证专业取证级不可恢复。SSD 磨损均衡、NTFS 日志、系统缓存、杀毒软件、云同步目录都可能留下副本。本项目实现的是教学意义上的覆写、同步、重命名和删除，并在 UI 和报告中说明限制。

问：为什么文件粉碎不支持批量目录？
答：粉碎是高风险不可逆操作。为了避免误操作，当前只支持用户手动选择单个普通文件，拒绝目录、符号链接、不存在路径和非法覆写次数。

问：为什么提供 1/3/7 次覆写？
答：用于演示不同安全级别和耗时的权衡。次数越多耗时越长，但在 SSD 等场景下仍不能等同于专业取证级擦除。
```

- [ ] **Step 4: 规则扩展问题**

答辩问答：

```text
问：为什么使用 JSON 规则文件？
答：清理路径、匹配模式、排除项、风险等级、最小文件年龄和默认勾选状态都放在 JSON 中，避免把路径硬编码进扫描逻辑。后续新增软件缓存只需要扩展规则，并经过校验。

问：无效规则会不会导致程序崩溃？
答：不会。规则加载时会校验路径、风险等级和匹配模式。无效规则会记录错误并跳过，扫描继续执行。

问：如何保证扩展规则不误删重要数据？
答：规则需要配置 category、risk、exclude、min_age_days 和 default_on。涉及系统目录、注册表、账号数据或隐私数据的规则必须标高风险并默认关闭。
```

---

### Task 5: 最终交付清单

**Files:**
- Modify: `docs/实训报告.md`

- [ ] **Step 1: 核对最终交付物**

最终交付清单：

```text
1. GitHub Release：GoCleaner-v1.0.0-windows-amd64.exe
2. 源码：Go + Wails + React + TypeScript
3. 配置：configs/cleaner_rules.json
4. 文档：docs/GoCleaner可行性计划.md
5. 文档：docs/系统分析文档.md
6. 文档：docs/系统设计文档.md
7. 文档：docs/实现与测试文档.md
8. 文档：docs/实训报告.md
9. 截图：docs/images/01-main-ui.png
10. 截图：docs/images/02-confirm-dialog.png
11. 截图：docs/images/03-result-states.png
12. 截图：docs/images/04-test-output.png
```

- [ ] **Step 2: 核对验收标准**

验收标准：

```text
1. 程序可以启动。
2. 可以完成扫描。
3. 可以展示扫描结果。
4. 可以清理低风险文件。
5. 可以记录操作日志。
6. 可以演示插件扫描。
7. 可以演示注册表无效项扫描和备份。
8. 可以演示文件粉碎。
9. 有测试记录。
10. 有完整报告。
11. GitHub Release 自动生成 exe 附件。
```

---

## 自检结果

- Spec coverage：已覆盖打包 exe、GitHub 自动打包发布、3-5 分钟演示流程、安全性答辩、注册表答辩、文件粉碎答辩和规则扩展答辩。
- Placeholder scan：没有 `TBD`、`TODO`、`implement later`、`fill in details` 等占位内容。
- Type consistency：workflow 使用当前项目已有的 Go、Node、Wails、frontend npm scripts 和 `build/bin/GoCleaner.exe` 输出路径。

Plan complete and saved to `docs/superpowers/plans/2026-06-13-day20-packaging-defense.md`. Two execution options:

1. Subagent-Driven (recommended) - dispatch a fresh subagent per task, review between tasks, fast iteration
2. Inline Execution - execute tasks in this session using executing-plans, batch execution with checkpoints

