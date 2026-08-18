# HDU 课表自动化编排助手

面向杭州电子科技大学选课场景的本地排课工具。普通用户只需要运行一个 exe，即可完成课程数据导出、个人课表导入、离线模拟排课、候选方案生成和课表导出。

当前版本的用户入口：

```text
HDU-Auto-Scheduling-Script.exe
```

## 快速开始

1. 下载 release 包并解压到一个独立文件夹。
2. 双击 `HDU-Auto-Scheduling-Script.exe`。
3. 程序会自动打开本地浏览器页面。
4. 如果输出目录没有 `course.json`，会进入课程导出页。
5. 如果输出目录已有 `course.json`，会直接进入排课助手。
6. 如果同时存在 `personal-schedule.json`，会自动作为个人课表底板导入并锁定。

`personal-schedule.json` 不是进入排课页的必要条件。缺少个人课表时仍可先排课，也可以在排课页点击“导出/更新课程”返回导出页补导。

## 仓库结构

本项目保持为一个 GitHub monorepo，用户只需要克隆一个仓库：

| 目录 | 作用 |
| --- | --- |
| 根目录、`main.go`、`scheduler.html` | 主排课助手和统一启动入口 |
| `school/` | 教务登录、全校课程和个人课表导出 |
| `HDU-Smart-Course-Agent/` | 独立 Go module，负责把目标课表转换为 KillCourse 执行准备 |
| `scripts/` | 构建、发布包、自检和 UI/API 验收脚本 |

Smart Agent 虽然保留独立 module 和独立可执行文件，但属于本项目的一部分，不需要拆成第二个 GitHub 仓库。`HDU-KillCourse-main` 是外部依赖目录，默认位于本机另一个目录，不复制进本仓库。

## 输出目录与启动规则

release 版默认把 `course.json`、`personal-schedule.json`、目标课表和当前课表写入 exe 所在目录，不受你从哪个快捷方式或工作目录启动的影响。开发/测试进程默认使用工作目录；也可以通过环境变量 `HDU_OUTPUT_DIR` 指定一个绝对或相对目录，所有读写会统一使用该目录。

因此，release 包中的数据文件应与 `HDU-Auto-Scheduling-Script.exe` 放在同一个目录。排课页导出的 `target-schedule.json` 和 `hdu-current-timetable.json` 也会写入同一输出目录。

| 输出目录文件 | 启动结果 |
| --- | --- |
| 没有 `course.json` | 进入课程导出页 |
| 有 `course.json`，没有 `personal-schedule.json` | 进入排课页，可手动导入或补导个人课表 |
| 有 `course.json` 和 `personal-schedule.json` | 进入排课页，并自动导入个人课表底板 |

## 主要功能

- 本地读取输出目录中的全校课表 `course.json`。
- 缺少 `course.json` 时，可通过导出页联网导出课程数据。
- 支持从任务落实 Excel 自动转换生成 `course.json`。
- 支持自动读取 `personal-schedule.json`，作为现有个人课表底板。
- 支持手动导入个人/班级 JSON 课表。
- 支持课程搜索、加入、移除、锁定、冲突提示和单双周展示。
- 支持课程级、时间级、方案级约束。
- 支持候选课表估算、生成、翻页预览、收藏、删除和导入显示。
- 候选生成默认在 Web Worker 中执行，减少页面卡顿。
- 候选为 0 时会显示原因诊断。
- 候选方案会显示学分、退课数、早八、晚课、全天无课和约束命中解释。
- 支持导出当前课表 JSON 和完整课表截图。

## 数据文件

| 文件 | 作用 |
| --- | --- |
| `course.json` | 全校课程数据，是进入排课页的核心数据 |
| `personal-schedule.json` | 个人课表数据，可作为已选底板自动导入 |
| `*.xlsx` | 任务落实课程导出的 Excel，可用于生成 `course.json`；当前不解析旧式 `.xls` |
| `samples/course.sample.json` | release 包内的全校课表示例数据 |
| `samples/personal-schedule.sample.json` | release 包内的个人课表示例数据 |

课程数据会统一归一化为 schema v1。源码仓库中的字段说明见 [docs/COURSE_SCHEMA.md](docs/COURSE_SCHEMA.md)，release 包中对应文件为 `COURSE_SCHEMA.md`。

启动时会显式检查当前数据目录：已有可解析的 `course.json` 优先使用；只有缺少课程 JSON，或 JSON 缺少有效学分且同目录存在 `.xlsx` 时，才会从 Excel 生成或修复文件。启动后的 `GET /api/status` 和 `GET /api/course` 只读文件，不会因为刷新页面创建备份或改写课程数据。

## 使用建议

- 第一次使用时，建议先通过导出页生成 `course.json`。
- 如果个人课表导出失败，也可以先进入排课页继续做模拟排课。
- 导入个人课表后，默认会作为“已有课程底板”，用于统计需要退选的课程。
- 锁定课程表示该教学班必须保留，生成候选方案时不会被替换。
- 未锁定的已选课程只表示“当前已加入预览”，生成候选方案时仍可被替换或退选。
- 退课有风险，后续若接入抢课执行器，应优先使用保守或安全替换策略。

## 本地开发

运行统一助手：

```bash
go run .
```

开发调试时仍可单独运行旧导出器入口：

```bash
go run ./cmd/course-exporter
```

程序会自动打开本地浏览器页面。若只想启动服务、不自动打开浏览器：

```bash
set HDU_NO_BROWSER=1
go run .
```

## 打包 exe

直接构建统一 exe：

```bash
go build -buildvcs=false -o dist/HDU-Auto-Scheduling-Script.exe .
```

生成可分发 release 包：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1
```

脚本会生成：

```text
release/HDU-Auto-Scheduling-Script-v版本号/
release/HDU-Auto-Scheduling-Script-v版本号.zip
```

release 包包含统一 exe、使用说明、课程 schema、版本文件、manifest 和测试样例数据。

若需要把排课结果转换为 `HDU-KillCourse` 执行计划，release 包内还包含 Smart Agent 的完整运行说明和日志 schema：

```text
smart-agent/HDU-Smart-Course-Agent.exe
smart-agent/README.md
smart-agent/SMART_AGENT_QUICKSTART.md
smart-agent/docs/EXECUTION_LOG_SCHEMA.md
```

`HDU-Smart-Course-Agent` 默认只生成执行计划、dry-run、授权票据和启动包，不会自动执行真实选课或退课。
Smart Agent 会自动发现保存的目标课表；个人课表刷新通过主站已登录的内存 exporter 会话完成，支持手动刷新和默认 60 秒（可调整为 10–7200 秒）的自动刷新。解析 KillCourse 日志发现选课或退课成功后也会立即刷新。主站重启后需要重新登录，刷新失败会保留上一次成功快照。当前适配独立的 `HDU-KillCourse-main` 目录（`v1.4.9`），不会与本项目源码合并，目录位置可在 Smart Agent 设置中配置。

## 测试与自检

### release 包自检

如果你拿到的是已经解压好的 release 包，优先运行下面这些自检命令：

```powershell
powershell -ExecutionPolicy Bypass -File scripts\release-main-smoke.ps1
powershell -ExecutionPolicy Bypass -File scripts\smart-agent-e2e.ps1
node scripts\smart-agent-ui-smoke.js
```

其中 `release-main-smoke.ps1` 会直接启动 release 包里的 `HDU-Auto-Scheduling-Script.exe`，验证“没有 `course.json`”和“只有 `course.json`、没有 `personal-schedule.json`”两个启动分支。Smart Agent 的两个脚本会使用临时目录，不会写入真实 `HDU-KillCourse/config.json`，也不会执行真实选课或退课。

### 源码仓库测试

固定测试数据在源码仓库位于 [testdata](testdata)，在 release 包内位于 `samples/`：

- `course.sample.json`
- `personal-schedule.sample.json`

源码仓库说明见 [docs/TEST_DATA.md](docs/TEST_DATA.md)，release 包说明见 `TEST_DATA.md`。

排课 Worker 冒烟测试：

```bash
node scripts/scheduler-worker-smoke.js
```

Go 测试：

```bash
go test -buildvcs=false ./...
```

Smart Agent 是仓库中的独立 Go module，根 module 的测试不会自动进入该目录；需要单独运行：

```powershell
Push-Location HDU-Smart-Course-Agent
go test -buildvcs=false ./...
Pop-Location
```

Smart Agent API 级完整链路验收：

```powershell
powershell -ExecutionPolicy Bypass -File scripts\smart-agent-e2e.ps1
```

该脚本会临时启动 `HDU-Smart-Course-Agent.exe`，使用 `tmp-smart-agent-e2e/` 中的临时 Scheduler 和假 KillCourse 目录验证“生成计划 -> 写入临时 config -> dry-run -> 授权 -> 启动包 -> 日志解析 -> 备选推荐”。它不会写入真实 `HDU-KillCourse/config.json`，也不会执行真实选课或退课。

Smart Agent UI 资源冒烟测试：

```bash
node scripts/smart-agent-ui-smoke.js
```

该脚本会临时启动 Smart Agent，检查页面 HTML/CSS/JS 和关键状态 API 是否可用。若本机 Edge/Chrome 支持命令行截图，会额外尝试保存首屏截图；截图失败只作为诊断提示，不影响资源/API 冒烟结果。

主站浏览器 UI 验收：

```powershell
go build -buildvcs=false -o "$env:TEMP\HDU-Auto-Scheduling-Script.exe" .
$env:HDU_MAIN_EXE = "$env:TEMP\HDU-Auto-Scheduling-Script.exe"
node scripts\main-ui-acceptance.js
```

该脚本在临时目录启动主程序，验证无数据、只有课程库、课程库加个人课表三种入口状态；每种状态都在桌面和移动视口检查入口跳转、课程搜索、加入/锁定/删除、个人课表自动锁定、清除底板、13 节课表、截图、横向溢出和控件遮挡。设置 `HDU_UI_KEEP_ARTIFACTS=1` 可保留截图和临时日志。

### 本地模拟教务系统验收

学校接口不可用时，以下命令可在本机模拟登录、课程查询和个人课表查询，
并把导出的结果继续送入排课和 Smart Agent 验证。模拟服务只监听
`127.0.0.1`，不使用真实账号、密码或 Cookie：

```powershell
powershell -ExecutionPolicy Bypass -File scripts\testlab-acceptance.ps1
```

场景说明和保留诊断输出的方法见 [docs/TEST_DATA.md](docs/TEST_DATA.md)。

课表截图导出冒烟测试需要先启动本地服务：

```bash
set HDU_NO_BROWSER=1
go run .
node scripts/screenshot-smoke.js
```

## Git 忽略规则

以下文件或目录默认不提交到 Git：

- `course.json`
- `personal-schedule.json`
- Excel 文件
- `dist/`
- `release/`
- exe 文件
- 临时截图测试目录

需要分发时，请使用 release 脚本生成 zip。
