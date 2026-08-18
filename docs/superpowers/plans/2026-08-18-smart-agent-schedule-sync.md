# Smart Agent Schedule Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 自动导入保存的目标课表，并让 Smart Agent 通过主站内存教务会话定时或手动刷新当前个人课表。

**Architecture:** 主站 `school.Service` 在成功登录后保留 exporter 会话，并提供只刷新个人课表的本地 API。Smart Agent 只调用 loopback API、读取主站输出、写入现有 live snapshot/diff；目标课表通过固定文件名和明确的 target 导出文件名自动发现，current 导出文件不会被当作目标课表。密码不落盘，CDP 不在本次范围内。

**Tech Stack:** Go `net/http`、现有 `school` exporter、Smart Agent Go HTTP handlers、原生 browser JavaScript/CSS、现有 Node UI smoke。

**Spec:** `docs/superpowers/specs/2026-08-18-smart-agent-schedule-sync-design.md`

## Global Constraints

- 默认自动刷新间隔为 60 秒，允许 10 到 7200 秒整数值。
- 只接受 loopback 主站地址，默认 `http://127.0.0.1:6789`。
- 不保存教务账号、密码或浏览器 Cookie。
- 刷新失败保留上一次成功快照，并显示失败原因。
- 目标课表自动发现必须兼容 `target-schedule.json`、`hdu-target-timetable.json` 和 `hdu-target-timetable-*.json`，并明确忽略 `hdu-current-timetable*.json`。
- 任何生产代码必须先有一个被观察到的失败测试。

---

### Task 1: Preserve the authenticated exporter session

**Files:**
- Modify: `school/school.go`
- Modify: `school/export.go`
- Test: `school/export_test.go`
- Test: `school/export_integration_test.go`

**Interfaces:**
- Add `(*Service).RefreshPersonalSchedule() (*ExportResult, error)`.
- `RunExport` stores the authenticated `*exporter` after login succeeds.
- A refresh without a stored session returns an error containing `请先完成登录`.

- [ ] **Step 1: Write the failing unit test for a missing session**

Add `TestRefreshPersonalScheduleRequiresSession` to `school/export_test.go`:

```go
func TestRefreshPersonalScheduleRequiresSession(t *testing.T) {
    service := NewService()
    _, err := service.RefreshPersonalSchedule()
    if err == nil || !strings.Contains(err.Error(), "请先完成登录") {
        t.Fatalf("RefreshPersonalSchedule() error = %v", err)
    }
}
```

- [ ] **Step 2: Run the focused test and verify the expected failure**

Run: `go test -buildvcs=false ./school -run TestRefreshPersonalScheduleRequiresSession -count=1`

Expected: FAIL because `RefreshPersonalSchedule` is not defined.

- [ ] **Step 3: Write the failing integration test for session reuse**

Extend the mock exporter in `school/export_integration_test.go` with a request counter and a test that runs the normal export once, calls `service.RefreshPersonalSchedule()`, asserts the second request reaches `/jw/personal`, and asserts the login endpoint was not called a second time.

- [ ] **Step 4: Run the integration test and verify it fails for the missing session reuse**

Run: `go test -buildvcs=false ./school -run 'TestRunExport.*RefreshPersonalSchedule' -count=1`

Expected: FAIL because the service does not expose or retain a refresh session.

- [ ] **Step 5: Implement the minimal session retention and refresh method**

Add an authenticated exporter field protected by the existing service mutex. Store it immediately after `exp.login` succeeds in `runExport`. Implement `RefreshPersonalSchedule` to acquire the stored exporter, use `beginRun`/`endRun`, call `exportPersonalSchedule(ExportRequest{})`, update service status, and return the result. Do not store credentials.

- [ ] **Step 6: Run the focused tests and verify they pass**

Run: `go test -buildvcs=false ./school -run 'TestRefreshPersonalScheduleRequiresSession|TestRunExport.*RefreshPersonalSchedule' -count=1`

Expected: PASS, with the mock login count unchanged during refresh.

### Task 2: Expose a main-site personal schedule refresh endpoint

**Files:**
- Modify: `main.go`
- Test: `main_test.go`

**Interfaces:**
- Add `POST /api/export/personal-schedule`.
- Reuse `school.Service.RefreshPersonalSchedule` and the existing `/api/export/status` response.
- Return HTTP `409` when no session or another export is active; return `202` when a refresh starts.

- [ ] **Step 1: Write the failing handler tests**

Add tests for method rejection and the no-session response:

```go
func TestHandlePersonalScheduleRefreshRequiresLogin(t *testing.T) {
    state := &appState{service: school.NewService()}
    req := httptest.NewRequest(http.MethodPost, "/api/export/personal-schedule", nil)
    rec := httptest.NewRecorder()
    handlePersonalScheduleRefresh(state)(rec, req)
    if rec.Code != http.StatusConflict {
        t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
    }
}
```

- [ ] **Step 2: Run the handler tests and verify RED**

Run: `go test -buildvcs=false . -run TestHandlePersonalScheduleRefresh -count=1`

Expected: FAIL because the handler and route do not exist.

- [ ] **Step 3: Implement the endpoint and register it**

Add `handlePersonalScheduleRefresh`, call `go state.service.RefreshPersonalSchedule()` after returning `202`, and register `/api/export/personal-schedule` beside the existing export routes. Reuse the existing status endpoint for polling and preserve its error text.

- [ ] **Step 4: Run the handler and root tests**

Run: `go test -buildvcs=false . -run TestHandlePersonalScheduleRefresh -count=1`
Run: `go test -buildvcs=false ./...`

Expected: PASS.

### Task 3: Add target-schedule discovery and persisted refresh settings

**Files:**
- Modify: `HDU-Smart-Course-Agent/main.go`
- Test: `HDU-Smart-Course-Agent/main_test.go`

**Interfaces:**
- Extend `AgentSettings` with `MainBaseURL`, `AutoRefresh`, and `RefreshIntervalMinutes`.
- Add `GET /api/target-schedule`.
- Add target metadata to `StatusResponse`: selected path, existence, modified time, and count.
- Add helpers `discoverTargetSchedule(paths)`, `loadTargetSchedule(paths)`, and `validateMainBaseURL(string) error`.

- [ ] **Step 1: Write failing target discovery tests**

Add tests using `t.TempDir()` that create a valid `target-schedule.json`, an ignored `hdu-current-timetable-*.json`, a valid `hdu-target-timetable-*.json`, and a malformed newer target candidate. Assert the fixed file wins, then assert the newest valid target file is selected when the fixed file is absent.

- [ ] **Step 2: Run the target tests and verify RED**

Run: `Push-Location HDU-Smart-Course-Agent; go test -buildvcs=false . -run 'TestDiscoverTargetSchedule|TestLoadTargetSchedule' -count=1; Pop-Location`

Expected: FAIL because target discovery helpers and response fields do not exist.

- [ ] **Step 3: Implement discovery and settings validation**

Scan only the two approved directories and the approved file name patterns. Sort legacy candidates by modification time descending, skip malformed files, and return warnings. Default settings to `http://127.0.0.1:6789`, `AutoRefresh=true`, and `RefreshIntervalMinutes=15`; clamp or reject values outside 5–120 and reject non-loopback URLs.

- [ ] **Step 4: Implement `GET /api/target-schedule` and include metadata in status/settings responses**

Return an empty successful response for no candidate, normalized items for a valid candidate, and warnings for skipped malformed files. Do not write or mutate the discovered target file.

- [ ] **Step 5: Run Smart Agent unit tests**

Run: `Push-Location HDU-Smart-Course-Agent; go test -buildvcs=false ./...; Pop-Location`

Expected: PASS.

### Task 4: Bridge Smart Agent refreshes to the main exporter

**Files:**
- Modify: `HDU-Smart-Course-Agent/main.go`
- Test: `HDU-Smart-Course-Agent/main_test.go`

**Interfaces:**
- Add `POST /api/live-schedule/refresh`.
- Add `refreshFromMain(ctx, baseURL string, paths paths) (LiveScheduleResponse, error)`.
- The endpoint returns the existing `LiveScheduleResponse` shape and writes the same live snapshot/diff files as manual import.

- [ ] **Step 1: Write a failing loopback bridge test**

Start an `httptest.Server` implementing `/api/export/personal-schedule` and `/api/export/status`, place a valid `personal-schedule.json` in a temporary scheduler directory, call the handler, and assert it returns `ok: true`, `source: "main-exporter"`, and a populated `syncedAt`.

- [ ] **Step 2: Run the bridge test and verify RED**

Run: `Push-Location HDU-Smart-Course-Agent; go test -buildvcs=false . -run TestLiveScheduleRefresh -count=1; Pop-Location`

Expected: FAIL because the refresh endpoint does not exist.

- [ ] **Step 3: Implement bounded polling and snapshot reuse**

Use an HTTP client with a short per-request timeout, call the main refresh endpoint, poll `/api/export/status` until `phase` is `success` or `error`, then load the local personal schedule and pass it through the existing live snapshot writer. Stop after a bounded two-minute deadline and return the last known snapshot in the response when refresh fails.

- [ ] **Step 4: Enforce the loopback boundary and preserve failures**

Reject non-loopback `MainBaseURL` values before any request. Never delete `personal-schedule-live.json` or `live-schedule-sync.json` on errors. Include the error in the response and status.

- [ ] **Step 5: Run Smart Agent tests**

Run: `Push-Location HDU-Smart-Course-Agent; go test -buildvcs=false ./...; Pop-Location`

Expected: PASS.

### Task 5: Add automatic target import and refresh controls to the UI

**Files:**
- Modify: `HDU-Smart-Course-Agent/web/index.html`
- Modify: `HDU-Smart-Course-Agent/web/app.js`
- Modify: `HDU-Smart-Course-Agent/web/styles.css`
- Test: `scripts/smart-agent-ui-smoke.js`

**Interfaces:**
- `refreshStatus()` loads `/api/target-schedule` and `/api/settings`.
- UI calls `POST /api/live-schedule/refresh` for manual and automatic refreshes.
- Settings are saved through the existing `/api/settings` endpoint.

- [ ] **Step 1: Add failing UI assertions**

Assert that the page contains `target-source`, `target-updated-at`, `refresh-live`, `auto-refresh`, `refresh-interval`, `last-refresh-at`, and `next-refresh-at`, and that a discovered target payload renders without selecting a file.

- [ ] **Step 2: Run UI smoke and verify RED**

Run: `node scripts/smart-agent-ui-smoke.js`

Expected: FAIL because the new controls and automatic target state do not exist.

- [ ] **Step 3: Implement automatic target initialization**

On `refreshStatus`, request `/api/target-schedule`; when it has items and no manual target override is active, set `state.targetPayload`, target metadata, and reset plan state. Keep the file picker as an explicit override.

- [ ] **Step 4: Implement refresh state and timer**

Add one timer and one in-flight promise guard. Render last successful `sync.syncedAt`, next scheduled refresh, source, and error text. Start an immediate refresh only when the snapshot is missing or older than the configured interval; otherwise schedule the next tick. Reset the timer after manual or automatic success.

- [ ] **Step 5: Implement controls and responsive styling**

Add a manual refresh button, auto-refresh checkbox, and 5–120 minute numeric input. Use stable widths, visible focus, and a responsive control row that wraps at 390px without overlapping the course summary or file picker.

- [ ] **Step 6: Run the UI smoke**

Run: `node scripts/smart-agent-ui-smoke.js`

Expected: PASS at desktop `1440x1000` and mobile `390x844`, including target auto import and refresh-state assertions.

### Task 6: Documentation, release, and full verification

**Files:**
- Modify: `HDU-Smart-Course-Agent/README.md`
- Modify: `HDU-Smart-Course-Agent/SMART_AGENT_QUICKSTART.md`
- Modify: `docs/TEST_DATA.md`
- Modify: `scripts/smart-agent-ui-smoke.js` if release-specific assertions require it

- [ ] **Step 1: Document the first-login/session lifecycle**

State that the user must complete the main exporter login once, that the session is memory-only, that Smart Agent refreshes through loopback, and that restarting the main app requires a new login. Document target file names and the 15-minute default/5–120 minute range.

- [ ] **Step 2: Run all unit, integration, worker, and UI checks**

Run:

```powershell
go test -buildvcs=false ./...
Push-Location HDU-Smart-Course-Agent
go test -buildvcs=false ./...
Pop-Location
node scripts/scheduler-worker-smoke.js
node scripts/smart-agent-ui-smoke.js
powershell -ExecutionPolicy Bypass -File scripts/testlab-acceptance.ps1
```

Expected: PASS without real credentials or external教务 access.

- [ ] **Step 3: Build and verify the release package**

Run the existing release build script, then run release main smoke, Smart Agent E2E, and Smart Agent desktop/mobile UI smoke. Confirm the release package includes the updated Smart Agent assets and endpoint behavior.

- [ ] **Step 4: Record the external acceptance boundary**

Document that automated tests use loopback mocks; manual acceptance must log in through the main exporter, confirm personal schedule refresh, inspect timestamps, change the interval, trigger manual refresh, and verify failed refresh preserves the last good snapshot.
