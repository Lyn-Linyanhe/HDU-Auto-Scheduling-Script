# HDU 课表自动化编排助手

[![CI](https://github.com/Lyn-Linyanhe/HDU-Auto-Scheduling-Script/actions/workflows/ci.yml/badge.svg)](https://github.com/Lyn-Linyanhe/HDU-Auto-Scheduling-Script/actions/workflows/ci.yml)

面向杭州电子科技大学选课场景的本地排课工具。普通用户只需要运行一个 exe，即可完成课程数据导出、个人课表导入、离线模拟排课、候选方案生成和课表导出。

当前版本的用户入口：

```text
HDU-Auto-Scheduling-Script.exe
```

## 最新发布

- **v0.5.13**（2026-08-21）：自动化编排页 P3 收口——顶栏按钮中宽不折行；
  约束自定义表单桌面单行/窄屏降列；移动端候选工具栏粘底；导入未匹配条目展示明细（最多 8 条）。
- **v0.5.12**（2026-08-21）：自动化编排页（排课页）专项审核落地——课组默认只显示前 5 个教学班
  的问题已修（新增“查看全部 N 个教学班 / 收起”）；搜索输入加 180ms 防抖，缓解大课程库卡顿。
- **v0.5.11**（2026-08-21）：UX 交互收口——候选超规模确认改应用内弹层并支持键盘操作；估算/生成失败改为“横幅+重试”；两站移动端按钮热区提升至 ≥44px（主站验收新增弹层/横幅 DOM 契约断言）。
- **v0.5.10**（2026-08-21）：全项目 UX 打磨——主站与智能助手按钮/徽章/状态格对齐统一；
  智能助手顶部新增“实时容量”状态卡，课程库计数加“按可选教学班计”口径说明，容量失败区分
  “保留上次成功快照/回退本地快照”；主站“删除候选”改为本轮快照式撤销（恢复不再重新求解）。
- **v0.5.9**（2026-08-21）：实时人数（余量）功能按“方案 B（schema-first）”落地——新增 `GET /api/course/live-capacity`（读取实时快照，无快照时回退 `course.json` 并标记实时/快照来源与时间）与 `POST /api/course/live-capacity/refresh`（复用登录会话拉余量接口）；字段映射收敛到候选键表（rl=容量 / xkrs=选课人数 / skrs=授课人数 / syl=余量），真实抓包后只改这一处；前端新增“刷新实时容量”按钮与实时/快照双来源显示、失败原因保留上次成功快照；testlab 新增 `capacity-ok`/`capacity-fail` 验收场景。
- **v0.5.8**（2026-08-21）：抓包工具升级——`POST /api/course/live-capacity/capture` 现在输出并保存 `rowSchema`（响应首行每个字段的 键名/类型/样例），真实余量字段一眼可见，实时人数功能从抓包到回填的路更顺。
- **v0.5.7**（2026-08-21）：稳定强化与抓包诊断就绪——执行器登录过期自动重登+重试、蹲课瞬时失败保留+指数退避、执行过程实时状态上报、个人课表自动刷新失败指数退避、接口响应 shape 变更诊断回归；新增容索引“抓包诊断”工具 `POST /api/course/live-capacity/capture`（保存原始响应为 `capacity-capture-diagnosis.json`，供实时人数功能回填字段）。
- **v0.5.6**（2026-08-21）：Smart Agent 新增行政班课表：`/api/class-options` + `/api/class-schedule?className=` 按授课班级聚合展示；发布包新增 `SHA256SUMS.txt` 与 `.zip.sha256` 完整性校验。
- **v0.5.5**（2026-08-21）：Smart Agent 内置一键执行：授权票据通过后可直接在应用内选课/退课/蹲课（vendor 的 HDU-KillCourse v1.4.9，含 start/status/stop API、1.5s 状态轮询、执行的日志写入与成功后的个人课表自动刷新）。
- **v0.5.4**（2026-08-21）：排课页“删除候选后可恢复 / 重新生成视为新一轮”、导入 JSON 底板时提示未匹配条目、CI 无头浏览器启动加固、Smart Agent 计划接口凭据脱敏。
- 下载与发布说明：[GitHub Releases](https://github.com/Lyn-Linyanhe/HDU-Auto-Scheduling-Script/releases)
- Release 资产：`HDU-Auto-Scheduling-Script.exe`（主站）+ `smart-agent/HDU-Smart-Course-Agent.exe`（智能选课执行助手）+ 完整 ZIP。

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
| 根目录、`main.go` | 主排课助手的 Go 入口和统一 HTTP 服务 |
| `web/` | 主排课助手的内嵌 HTML、CSS、JavaScript 和 Worker |
| `school/` | 教务登录、全校课程和个人课表导出 |
| `HDU-Smart-Course-Agent/` | 独立 Go module，负责把目标课表转换为 KillCourse 执行准备，并支持授权后的内置一键执行（vendor `third_party/HDU-KillCourse`） |
| `cmd/` | 兼容导出器和仅供验收使用的本地 testlab 命令 |
| `scripts/` | 构建、发布包、自检和 UI/API 验收脚本 |
| `docs/` | 用户文档、仓库规范和带日期的工程记录 |
| `testdata/` | 不含真实账号和个人课表的确定性测试样例 |

Smart Agent 虽然保留独立 module 和独立可执行文件，但属于本项目的一部分，不需要拆成第二个 GitHub 仓库。`HDU-KillCourse-main` 是外部依赖目录，默认位于本机另一个目录，不复制进本仓库。

文档导航见 [docs/README.md](docs/README.md)。

### 仓库文件边界

源码仓库只提交源码、文档、测试样例和构建脚本。个人课表、登录配置、Cookie、运行日志、浏览器目录、exe、ZIP 和 `release/` 均为本地产物；可分发 ZIP 应作为 GitHub Release asset 上传。详细规则见 [docs/REPOSITORY_LAYOUT.md](docs/REPOSITORY_LAYOUT.md)。

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
- 支持手动导入个人/班级 JSON 课表，未匹配条目会明确提示数量。
- 支持课程搜索、加入、移除、锁定、冲突提示和单双周展示。
- 支持课程级、时间级、方案级约束。
- 支持候选课表估算、生成、翻页预览、收藏、删除、恢复和导入显示；删除全部候选后可一键恢复，重新生成会返回新一轮全部候选。
- 候选生成默认在 Web Worker 中执行，减少页面卡顿。
- 候选为 0 时会显示原因诊断。
- 候选方案会显示学分、退课数、早八、晚课、全天无课和约束命中解释。
- 支持导出当前课表 JSON 和完整课表截图。
- 智能选课执行助手（Smart Agent）可把目标课表一键转成执行计划，经 dry-run → 授权票据后在应用内直接选课/退课/蹲课，并实时显示每门课进度、随时停止。
- Smart Agent 支持按授课班级聚合展示行政班课表，并提供余量接口抓包诊断工具（`POST /api/course/live-capacity/capture`，输出 rowSchema）。
- Smart Agent 已按 schema-first 契约落地“实时人数/余量”：`POST /api/course/live-capacity/refresh` 拉取并写 `live-capacity.json`，`GET /api/course/live-capacity` 读快照、失败回退 `course.json` 并标记实时/快照；前端“刷新实时容量”一键拉取、显示来源与时间、失败保留上次成功快照。

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

开发调试兼容流程时仍可单独运行导出器入口：

```bash
go run ./cmd/course-exporter
```

该命令仅作为兼容和开发入口保留；release 用户应运行统一的 `HDU-Auto-Scheduling-Script.exe`。

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

`release/` 和 `dist/` 是本地构建输出，不会随 `git push` 上传。需要发布时，应从当前提交重新生成 ZIP，并将它作为与 `VERSION` 相同 tag 的 GitHub Release asset 单独上传。

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

#### 持续集成

GitHub Actions 会检查仓库布局，运行根目录和 Smart Agent 两个 Go module 的测试、排课 Worker smoke，以及确定性的本地教务系统验收。提交前可先运行布局门禁：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check.ps1
```

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
