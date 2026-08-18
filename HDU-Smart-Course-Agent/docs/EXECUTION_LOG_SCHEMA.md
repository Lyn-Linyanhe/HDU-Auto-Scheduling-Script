# execution-log.json schema

`execution-log.json` is the structured execution result file for HDU-Smart-Course-Agent.

The first implementation should parse `HDU-KillCourse/log_files/app.log` after a manual `run-killcourse.bat` run. It should not require modifying HDU-KillCourse yet.

## Source Signals

HDU-KillCourse currently writes logs with this shape:

```text
2026/07/08 12:00:00.000000 [INFO] 正在处理课程: (2026-2027-1)-A0001001-01
2026/07/08 12:00:01.000000 [INFO] 选课成功
2026/07/08 12:00:02.000000 [ERROR] 选课失败: ...
2026/07/08 12:00:03.000000 [INFO] 退课成功(可能？)
2026/07/08 12:00:04.000000 [ERROR] 退课失败：...
```

Useful source messages:

- `正在处理课程: <code>` starts a course action block.
- `课程名称: <name>` fills the course name.
- `上课时间: <time>` fills schedule text.
- `选课成功` marks a select action as success.
- `选课失败: <reason>` marks a select action as failed.
- `选课失败: 人数可能已满` maps to `failureType = "full"`.
- `退课成功(可能？)` marks a drop action as success with a warning.
- `退课失败：<reason>` marks a drop action as failed.
- `处理课程失败: <reason>` marks the active action as failed.
- `登录过期` maps to `failureType = "login-expired"`.
- `课程不存在` / `不存在` maps to `failureType = "not-found"`.
- `时间格式错误` / `学期格式错误` maps to `failureType = "config"`.
- Network or request errors should map to `failureType = "network"` when recognizable, otherwise `unknown`.

## Top-Level Shape

```json
{
  "schemaVersion": 1,
  "source": "hdu-killcourse-app-log",
  "generatedAt": "2026-07-08T12:10:00+08:00",
  "logPath": "E:/.../HDU-KillCourse/log_files/app.log",
  "planHash": "sha256...",
  "configHash": "sha256...",
  "ticketId": "abcdef1234567890",
  "summary": {
    "total": 2,
    "success": 1,
    "failed": 1,
    "pending": 0,
    "skipped": 0
  },
  "items": []
}
```

## Item Shape

```json
{
  "courseCode": "(2026-2027-1)-A0001001-01",
  "courseName": "示例课程",
  "action": "select",
  "status": "failed",
  "failureType": "full",
  "message": "选课失败: 人数可能已满",
  "rawLines": [
    "2026/07/08 12:00:00.000000 [INFO] 正在处理课程: (2026-2027-1)-A0001001-01",
    "2026/07/08 12:00:02.000000 [ERROR] 选课失败: 人数可能已满"
  ],
  "startedAt": "2026-07-08T12:00:00+08:00",
  "finishedAt": "2026-07-08T12:00:02+08:00"
}
```

## Enumerations

`action`:

- `select`
- `drop`
- `wait`
- `unknown`

`status`:

- `pending`
- `running`
- `success`
- `failed`
- `skipped`

`failureType`:

- `full`
- `login-expired`
- `not-found`
- `conflict`
- `config`
- `network`
- `unknown`
- empty string when status is not `failed`

## Parser Rules

1. Use `action-plan.json` or the generated config to map course code to expected action.
2. Start a new item when seeing `正在处理课程:`.
3. Attach following course info lines to the active item until another `正在处理课程:` appears.
4. Mark success or failure when seeing known terminal messages.
5. If an active item has no terminal message by end of file, mark it as `running` or `pending`.
6. Preserve raw relevant lines for debugging.
7. Never store account passwords, cookies, or full config contents in `execution-log.json`.

## First Parser Scope

The first parser only needs to support post-run parsing:

```text
KillCourse app.log
       ↓
Smart Agent parser
       ↓
execution-log.json
       ↓
UI result summary
```

Real-time log streaming and automatic fallback recommendations should come later.
