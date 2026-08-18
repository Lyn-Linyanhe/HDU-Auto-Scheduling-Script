# Smart Agent Schedule Sync Design

**Date:** 2026-08-18
**Status:** Approved for implementation

## Goal

让 HDU Smart Course Agent 在启动时自动发现并导入保存的目标课表，并通过主站已经建立的教务登录会话刷新当前个人课表。页面需要显示最近一次成功刷新时间、下一次自动刷新时间、刷新间隔和失败状态，同时保留手动刷新入口。

## Product Decisions

### Target schedule

- Smart Agent 启动和路径刷新时自动扫描 Agent 工作目录与排课助手目录。
- 优先读取固定文件名 `target-schedule.json` 和 `hdu-target-timetable.json`。
- 只兼容明确标记为目标课表的 `hdu-target-timetable-*.json`；排课助手导出的 `hdu-current-timetable*.json` 是当前课表，禁止自动当作目标课表。
- 自动导入只初始化目标课表；现有文件选择器仍可覆盖自动发现结果。
- UI 显示自动发现文件的路径、修改时间、课程数量和来源，避免用户误以为目标课表仍未导入。

### Current personal schedule

- 主站是唯一教务登录和解析入口，Smart Agent 不复制账号密码、不实现第二套教务解析器。
- 主站成功登录后在进程内保留带 cookie jar 的 exporter 会话；密码永不写入磁盘。
- 新增仅刷新个人课表的本地接口。该接口复用现有 exporter 会话，只请求个人课表，不重复导出全校课程。
- Smart Agent 通过 loopback HTTP 调用该接口，等待主站任务完成后读取 `personal-schedule.json`，写入现有 `personal-schedule-live.json` 和差异快照。
- 主站或登录会话重启后，刷新接口返回“请先在主站完成登录/导出”，Smart Agent 保留上一次成功快照并显示可操作错误。
- 不在本次范围内实现 CDP、浏览器 Cookie 复制、浏览器扩展或密码持久化。

### Refresh behavior

- 默认自动刷新间隔为 60 秒。
- 允许关闭自动刷新，并允许设置 10 到 7200 秒的整数间隔。
- 启动时若自动刷新开启且没有成功快照，或快照已经超过间隔，则尝试一次刷新；否则等待到下一次计划时间。
- 手动刷新立即执行，并重置下一次自动刷新倒计时。
- 同一时间只允许一个刷新请求。
- 刷新失败不清空上一次成功的个人课表；页面同时显示失败时间、失败原因和上一次成功时间。

## Architecture

### Main site exporter

`school.Service` owns the authenticated exporter session. `RunExport` stores the exporter after login succeeds. A new `RefreshPersonalSchedule` operation reuses that exporter and writes the existing `personal-schedule.json` output. The root main server exposes:

```text
POST /api/export/personal-schedule
GET  /api/export/status
```

The POST operation starts an asynchronous refresh and returns a clear error when there is no authenticated session or another export is running. The existing full export flow and exporter UI remain unchanged.

### Smart Agent bridge

Smart Agent settings gain:

```json
{
  "mainBaseURL": "http://127.0.0.1:6789",
  "autoRefresh": true,
  "refreshIntervalSeconds": 60
}
```

Only loopback main URLs are accepted. `POST /api/live-schedule/refresh` calls the main refresh endpoint, polls its existing status endpoint with a bounded timeout, loads the refreshed local personal schedule, and reuses the current live snapshot/diff writer. A successful response has the same `LiveScheduleResponse` shape as manual JSON import and uses `source: "main-exporter"` in the sync record.

### Target discovery API

Smart Agent adds:

```text
GET /api/target-schedule
```

The endpoint returns `exists`, `path`, `updatedAt`, normalized `items`, warnings, and the current status. Missing files are a successful empty state; malformed candidate files are reported as warnings and do not replace a valid older candidate.

## UI

The current personal schedule block gets a compact refresh control row:

- `立即刷新` button;
- `自动刷新` checkbox;
- numeric interval input in seconds;
- `上次刷新` and `下次刷新` timestamps;
- source/status text such as `主站教务会话`, `已同步`, `刷新失败，保留上次快照`.

The target schedule block shows `自动导入` when a discovered file is loaded and keeps `导入目标课表 JSON` for manual override. The existing workflow stages and plan generation semantics are unchanged.

The layout stays dense and operational: the refresh row uses stable control dimensions, visible keyboard focus, no nested decorative cards, and remains usable at the existing desktop and 390px mobile UI smoke viewports.

## Error and security behavior

- No username or password is written by either application.
- Smart Agent only calls a validated loopback main URL.
- A missing main service, expired session, non-success export status, malformed personal schedule, and timeout each produce a user-readable state without deleting the last good snapshot.
- Local mock tests cover the bridge; real教务登录 remains a manual acceptance boundary.

## Verification

- Go unit/integration tests cover session retention, refresh reuse, no-session errors, target discovery, loopback validation, and malformed files.
- Smart Agent UI smoke covers automatic target import, refresh controls, timestamp/status rendering, manual refresh wiring, and mobile geometry.
- Parsing a successful select/drop execution log requests one personal-schedule refresh; a failed refresh keeps the prior successful snapshot and remains retryable.
- Existing root, Smart Agent, worker, TestLab, and release checks remain green.
