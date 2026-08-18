# HDU-Smart-Course-Agent

HDU 智能选课执行助手是排课助手和 `HDU-KillCourse` 之间的上层编排项目。MVP 只生成执行计划，不执行真实选课。

本目录是 `HDU-Auto-Scheduling-Script` monorepo 中的独立 Go module，和根目录主排课助手一起发布；它不是需要单独上传的第二个 GitHub 项目。`HDU-KillCourse-main` 仍作为本机外部依赖使用，不复制到本目录。

## 运行

```bash
go run .
```

默认地址：

```text
http://127.0.0.1:6899
```

## 输入

- `course.json`：全校课表，用于生成备选教学班。
- `personal-schedule.json`：当前个人课表，用于计算退课差异。
- 目标课表 JSON：优先自动发现 `target-schedule.json`、`hdu-target-timetable.json`，也兼容按修改时间选择的 `hdu-target-timetable-*.json`。`hdu-current-timetable*.json` 是当前课表导出，不会被自动当作目标课表。

## 输出

- `action-plan.json`
- `HDU-KillCourse/config.json`
- `execution-approval.json`
- `run-killcourse.bat`
- `execution-runbook.md`
- `execution-package.json`
- `execution-log.json`：从 `HDU-KillCourse/log_files/app.log` 解析出的结构化执行结果，schema 见 `docs/EXECUTION_LOG_SCHEMA.md`
- `fallback-recommendations.json`：基于执行日志中失败的选课项，推荐同课程号备选教学班。

`config.json` 中：

- `"1"` 表示选课
- `"0"` 表示退课

这些输出文件都是本地运行产物，默认不应提交到 Git。

## 当前能力

- 检测排课助手和 `HDU-KillCourse` 目录。
- 支持在页面里手动保存项目路径，写入本地 `agent-settings.json`；后续状态检测、计划生成和 dry-run 都优先使用该路径。
- 自动读取保存的目标课表并显示来源、文件更新时间和课程数量；文件选择器仍可手动覆盖自动发现结果。
- 读取 `course.json`、`personal-schedule.json`，并显示已有的实时个人课表快照。
- 通过主站已经登录的教务 exporter 会话刷新个人课表；页面支持立即刷新、自动刷新和 10–7200 秒间隔设置，默认每 60 秒刷新一次。
- 解析 KillCourse 日志发现选课或退课成功后，页面会立即复用同一刷新入口同步个人课表；重复解析同一批成功日志不会重复触发刷新。
- 计算 `keep`、`select`、`drop`、`locked`。
- 生成同课程号备选教学班 `fallbackGroups`。
- 对退课、锁定冲突、目标课表重复课程等情况生成风险和校验结果。
- 在 `action-plan.json` 中输出 `validationIssues` 和 `explanations`，说明每门课为什么被选、退、保留或锁定。
- 默认适配独立的 `HDU-KillCourse-main` 目录（当前源码版本 `v1.4.9`）；本项目不复制或合并 KillCourse 源码，只通过其现有入口、`config.json` 和 `log_files/app.log` 集成，目录位置可在设置中配置。
- 读取已有 `HDU-KillCourse/config.json`，保留账号、cookie、蹲课、邮箱、开始时间、`user_agent`、`ClientBodyConfigEnabled` 和 `CrossGradeEnabled` 等非课程设置。
- 只替换 `config.json` 的 `course` 字段，并预览旧动作和新动作差异。
- 支持下载新的 `config.json`，也支持直接写入 `HDU-KillCourse/config.json`。
- 生成执行准备检查，提前校验登录配置、学年学期、课程动作、开始时间、蹲课间隔和退课风险。
- 提供 dry-run 执行控制台：只检查 KillCourse 入口、配置文件、动作数量和风险日志，不真实执行选课/退课。
- dry-run 会确认磁盘上的 `HDU-KillCourse/config.json` 是否已经与当前计划一致，并显示解析后的工作目录、配置文件、启动入口和 `log_files/app.log` 路径；若尚未勾选写入配置，会提示先写入再检查。
- dry-run 通过后，可输入确认短语生成本地 `execution-approval.json` 授权票据；票据只保存哈希、动作数量、路径和有效期，不保存账号密码或 cookie。
- 授权票据有效且与当前计划一致时，可生成 `run-killcourse.bat`、`execution-runbook.md` 和 `execution-package.json` 启动包；启动包需要用户手动运行，不会由本项目自动触发真实选退课。bat 启动 KillCourse 前还会先暂停一次，防止误触。
- 提供只读课程能力接口：`/api/course-options`、`/api/course-capacity`、`/api/class-schedule`；课程容量和人数来自本地 `course.json` 快照。`/api/course-capacity` 返回 `stale: true`、`sourceUpdatedAt`（快照文件更新时间或导出时间）和 `observedAt`（接口读取时间），不能视为教务实时数据。
- 支持在解析 `execution-log.json` 后生成失败课程备选推荐：只分析选课失败项，不自动改写 `config.json`，优先展示同课程号、无时间冲突、教师一致的教学班。

## 安全边界

MVP 不会真实执行选课、退课或蹲课。它只生成完整执行计划，并在退课项上标记高风险。执行控制台里的 dry-run、授权票据和启动包也只是本地安全准备；只有用户手动运行 `run-killcourse.bat` 并在 KillCourse 窗口按 Enter 后，才会进入真实执行。

已经支持手动运行 KillCourse 后只解析本次启动包记录的日志起始位置之后的 `HDU-KillCourse/log_files/app.log`，生成结构化执行结果。
失败课程备选推荐仍然只读：它只生成 `fallback-recommendations.json` 和页面建议，不会自动把备选课写入 KillCourse 配置，也不会触发真实选课。

个人课表刷新只允许调用本机 loopback 主站地址。首次使用前需要在主站完成教务登录；登录后的 exporter 会话只保存在主站进程内，不写入账号、密码或浏览器 Cookie。主站重启后需要重新登录，刷新失败时 Smart Agent 会保留上一次成功快照并显示失败原因。

## 测试

```bash
node --check web/app.js
go test -buildvcs=false ./...
go build -o HDU-Smart-Course-Agent.exe .
```

从主仓库根目录运行本模块测试：

```powershell
Push-Location HDU-Smart-Course-Agent
go test -buildvcs=false ./...
Pop-Location
```

从主项目根目录运行 API 级完整链路验收：

```powershell
powershell -ExecutionPolicy Bypass -File scripts\smart-agent-e2e.ps1
```

该脚本只使用临时 KillCourse 目录和模拟日志，不会写入真实 `HDU-KillCourse/config.json`，也不会执行真实选课、退课或蹲课。

从主项目根目录运行 UI 资源冒烟测试：

```bash
node scripts/smart-agent-ui-smoke.js
```

该脚本检查 Smart Agent 页面 HTML/CSS/JS 和关键状态 API；截图能力依赖本机 Edge/Chrome，失败时只作为诊断提示。
