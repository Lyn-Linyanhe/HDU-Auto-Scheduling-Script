# Stability and KillCourse Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (or inline TDD execution when the work remains tightly coupled). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复已确认的刷新、会话、快照和移动端问题，并让 Smart Agent 保留并兼容最新 `HDU-KillCourse-main` 配置与执行入口。

**Architecture:** 主站负责持有内存登录会话和个人课表刷新状态；Smart Agent 通过 loopback API 触发并轮询刷新，只有确认实时课表内容发生变化时才使现有执行计划失效。配置生成采用宽结构透传未知但受支持的 KillCourse 字段，执行前以绝对路径、上游配置规则和本次日志边界做检查。

**Tech Stack:** Go `net/http`/`encoding/json`/`os`、原生 JavaScript/CSS、现有 Go/Node 测试和 PowerShell 启动脚本。

**Spec:** 本目标来自用户要求和上一轮验收发现；现有同步设计见 `docs/superpowers/specs/2026-08-18-smart-agent-schedule-sync-design.md`。

## Global Constraints

- 不回退工作区已有用户改动，不把 `hdu-current-timetable.json` 自动当作目标课表。
- 刷新失败必须保留上一次成功课表和同步元数据；空课表是合法的成功快照，`null`/不可解析响应不是成功。
- 主站刷新状态必须在异步任务启动前发布为 `exporting`，Smart Agent 不得把旧 `success` 当作本次刷新结果。
- 只接受 loopback 主站地址；不保存账号密码或浏览器 Cookie。
- KillCourse 配置中的 `user_agent`、`ClientBodyConfigEnabled`、`CrossGradeEnabled` 必须在重写 `course` 时保留。
- 任何生产代码必须先有一个被观察到的失败测试。

---

### Task 1: Harden exporter refresh state and personal snapshots

**Files:**
- Modify: `school/export.go`
- Modify: `school/school.go`
- Modify: `school/browser_export.go`
- Test: `school/export_test.go`
- Test: `school/export_integration_test.go`
- Test: `school/browser_export_test.go`

**Interfaces:**
- `Service.StartPersonalScheduleRefresh` publishes `exporting` before launching its worker.
- 401/403 errors are classified by status, not Chinese message text, and clear the cached authenticated session.
- `personal-schedule.json` and diagnostics use atomic replacement; a JSON `null` response fails without changing the previous file.

- [ ] Add failing tests for pre-launch `exporting`, 401/403 session invalidation, null response preservation, and atomic personal snapshot visibility.
- [ ] Run the focused tests and observe each expected failure.
- [ ] Add the typed HTTP status error, synchronous status publication, null validation, and shared atomic writer.
- [ ] Run the focused tests and the complete root Go suite.

### Task 2: Make Smart Agent refresh semantics reliable

**Files:**
- Modify: `HDU-Smart-Course-Agent/main.go`
- Modify: `HDU-Smart-Course-Agent/web/app.js`
- Modify: `HDU-Smart-Course-Agent/web/index.html`
- Modify: `HDU-Smart-Course-Agent/web/style.css`
- Test: `HDU-Smart-Course-Agent/main_test.go`
- Test: `scripts/smart-agent-ui-smoke.js`

**Interfaces:**
- The main bridge uses the request context deadline instead of a 5-second client timeout.
- Refresh failures return an HTTP error status with a JSON body; successful responses remain unchanged.
- A successful refresh compares the previous and new live snapshot signature and only resets a plan when the schedule changed.
- Manual import failures remain visible and reset the file input so the same invalid file can be selected again.
- A 390px viewport contains the full Smart Agent controls without horizontal overflow.

- [ ] Add failing handler and UI smoke assertions for HTTP error status, unchanged-plan preservation, changed-plan invalidation, and persistent import errors.
- [ ] Run focused tests/smoke and observe failure.
- [ ] Implement bounded bridge timeout, status codes, live signatures, UI error state, and responsive controls.
- [ ] Run Smart Agent Go tests, syntax checks, and desktop/mobile UI smoke.

### Task 3: Preserve latest KillCourse configuration and execution boundaries

**Files:**
- Modify: `HDU-Smart-Course-Agent/main.go`
- Modify: `HDU-Smart-Course-Agent/main_test.go`
- Modify: `HDU-Smart-Course-Agent/web/app.js`
- Modify: `HDU-Smart-Course-Agent/SMART_AGENT_QUICKSTART.md`

**Interfaces:**
- `KillCourseConfig` round-trips `user_agent`, `ClientBodyConfigEnabled`, and `CrossGradeEnabled`.
- Existing config values are preserved while only `course` and matching term fields are replaced.
- CAS/NewJW credentials are considered complete only when username and password are paired.
- Resolved KillCourse directory, config path, entry point, and log path are shown before execution.

- [ ] Add failing round-trip and paired-credential tests using a fixture matching `E:/fascinating project/HDU-KillCourse-main/config/config.go`.
- [ ] Run focused tests and observe failure from dropped fields/weak validation.
- [ ] Extend the config structure, preserve optional fields, and strengthen execution readiness checks.
- [ ] Add a log offset or run-boundary marker so parsing does not mix historical `app.log` entries with the current run.
- [ ] Run Smart Agent tests and inspect generated config against the new KillCourse source schema.

### Task 4: Rebuild, replace stale processes, and verify the integrated runtime

**Files:**
- Modify if needed: `scripts/build-release.ps1`, `scripts/smart-agent-e2e.ps1`, `scripts/smart-agent-ui-smoke.js`
- Build: `HDU-Auto-Scheduling-Script.exe`, `HDU-Smart-Course-Agent/HDU-Smart-Course-Agent.exe`

- [ ] Run root and Smart Agent Go tests, Node syntax checks, worker smoke, Smart Agent E2E, and UI smoke.
- [ ] Build both executables from current sources so embedded web assets are current.
- [ ] Stop the stale `hdu-smart-agent-runtime.exe` and start the newly built Smart Agent from the workspace with the resolved `HDU-KillCourse-main` path.
- [ ] Verify ports `6789` and `6899`, API status, 4449 course records, 10 teaching-class personal schedule, target discovery semantics, and KillCourse path.
- [ ] Verify desktop and 390px mobile UI, including failed-refresh retention and unchanged-plan behavior.

### Task 5: Final audit and documentation

- [ ] Review the diff for unrelated changes and preserve all existing user files.
- [ ] Update user-facing docs with the new KillCourse path/field preservation and restart/session lifecycle.
- [ ] Record any external-school-login limitation separately from code defects.
- [ ] Report exact running URLs, build timestamps, test results, and remaining manual acceptance boundary.
