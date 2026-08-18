# Schedule Foundation and UI Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make course data initialization deterministic and read-only at request time, then verify the three entry states and the key browser surfaces on desktop and mobile viewports.

**Architecture:** Keep Excel repair in the existing `school.Service`/data boundary and make HTTP GET handlers consume already-available files. A small Node/CDP acceptance harness will start an isolated built executable, exercise no-data/course-only/course-plus-personal workspaces, and assert both DOM state and geometry without adding a browser dependency.

**Tech Stack:** Go standard library, existing `excelize` parser, Node.js, system Edge/Chrome CDP, PowerShell process orchestration.

**Spec:** `docs/superpowers/specs/2026-07-13-schedule-data-integrity-design.md`, `docs/superpowers/specs/2026-07-13-local-teaching-system-testlab-design.md`

## Global Constraints

- Preserve unrelated user changes in the dirty worktree.
- A valid existing `course.json` with usable credits takes precedence over Excel.
- Excel repair is allowed only during explicit/startup initialization, never as a side effect of a GET status or course request.
- The first Excel implementation supports `.xlsx`; documentation must not claim `.xls` until a real parser path exists.
- No real account, password, cookie, or KillCourse execution is used by acceptance tests.
- UI acceptance must cover no data, course only, and course plus personal schedule, plus desktop and mobile viewport geometry.

---

### Task 1: Lock the read-only status contract

**Files:**
- Modify: `main_test.go`
- Modify: `main.go`
- Modify: `school/school.go`
- Modify: `school/excel.go`
- Modify: `docs/COURSE_SCHEMA.md`
- Modify: `README.md`

**Interfaces:**
- `handleStatus` and `handleCourse` remain GET handlers but may only read files.
- A data initialization/repair function owns Excel conversion and returns the selected source.
- The status response continues to expose `ready`, `count`, `courseName`, and personal-schedule state.

- [ ] **Step 1: Write the failing test**

Add a test that creates a creditless `course.json` and a same-directory `.xlsx`, calls `handleStatus` and `handleCourse`, and asserts that neither request creates/replaces `course.json` or creates a backup. Add a separate test for an existing valid-credit JSON proving a GET does not consult or rewrite the Excel file.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test -run 'TestHandle(Status|Course).*ReadOnly' ./...`

Expected: FAIL because the current GET handlers call `school.EnsureCourseFile`.

- [ ] **Step 3: Implement the smallest data-boundary change**

Split the existing ensure/repair behavior into an explicit initializer and a read-only loader. Keep valid JSON precedence, preserve source diagnostics, and make handlers return a clear not-ready response when the file is absent or unusable. Do not introduce a new storage format without updating the schema document and fixtures.

- [ ] **Step 4: Run the focused and package tests**

Run: `go test -run 'TestHandle(Status|Course).*ReadOnly' ./...`

Run: `go test -buildvcs=false ./...`

Expected: PASS with no files changed inside the temporary test directory after GET requests.

- [ ] **Step 5: Update documentation and record the contract**

Document the `.xlsx`-only behavior, JSON precedence, initialization boundary, and browser canonical fields in `README.md` and `docs/COURSE_SCHEMA.md`.

### Task 2: Validate real course fixtures and time/credit fidelity

**Files:**
- Modify: `school/sample_data_test.go`
- Modify: `school/export_test.go`
- Modify: `school/excel.go`
- Modify: `testdata/course.sample.json`
- Modify: `docs/TEST_DATA.md`

**Interfaces:**
- Existing `school.CoursePayload`/canonical normalization remains compatible with `shared.js`.
- Course identifiers use display teaching-class numbers when raw internal IDs are present.
- Credits retain quarter values such as `0.25` and schedule parsing retains sections through 13 with odd/even week information.

- [ ] **Step 1: Add failing fixture assertions**

Assert the real `dist` workbook can be parsed, at least one course has a `0.25` credit value, a display code includes `A0512150-04` rather than the internal ID, and period 10/13 values are represented as `18:30-19:15` and `21:00-21:45` by the canonical schedule parser.

- [ ] **Step 2: Run the focused tests and confirm RED**

Run: `go test ./school -run 'Test(Real|Sample).*Course|Test.*Schedule' -v`

Expected: FAIL only for behavior still missing, not for malformed test setup.

- [ ] **Step 3: Implement parser/normalizer fixes**

Use structured workbook cell access and existing canonical helpers; do not parse the Excel XML with string operations. Keep raw source fields for diagnostics while exposing stable `displayCode`, `courseCode`, `xf`, and schedule data to the browser.

- [ ] **Step 4: Run the full data test set**

Run: `go test ./school -v`

Expected: PASS, including credit preservation and real workbook coverage when the workbook exists; fixture tests must skip only with an explicit documented missing-fixture reason.

### Task 3: Add the isolated main-app UI acceptance harness

**Files:**
- Create: `scripts/main-ui-acceptance.js`
- Modify: `README.md`
- Modify: `scripts/testlab-acceptance.ps1`

**Interfaces:**
- The script accepts `HDU_MAIN_EXE` and optional `HDU_UI_KEEP_ARTIFACTS` environment variables.
- It starts an isolated executable workspace and reports JSON results for `no-data`, `course-only`, and `course-and-personal`.
- It uses CDP only through the system Edge/Chrome and fails on missing screenshots, JS errors, horizontal overflow, or overlapping primary controls.

- [ ] **Step 1: Write the failing harness assertions**

Start with a harness that expects the root page to settle at `/exporter/` for no data and `/scheduler.html` for the other two scenarios, verifies the exporter form, scheduler course list/timetable/base summary, and captures 1440px and 390px screenshots.

- [ ] **Step 2: Run it against the current build to identify real failures**

Run: `node scripts/main-ui-acceptance.js`

Expected: the script either passes current covered states or reports concrete missing selectors/layout issues with scenario and viewport names.

- [ ] **Step 3: Implement only the needed page/test fixes**

Keep selectors stable, expose useful accessible names where needed, and fix actual overflow/overlap defects in `index.html`, `scheduler.html`, `cmd/course-exporter/web/*`, or `styles.css`.

- [ ] **Step 4: Run desktop and mobile acceptance**

Run: `node scripts/main-ui-acceptance.js`

Expected: all three scenarios pass at both viewports; artifacts are retained when `HDU_UI_KEEP_ARTIFACTS=1` for visual inspection.

### Task 4: Integrate phase-1 gates and regression checks

**Files:**
- Modify: `README.md`
- Modify: `docs/TEST_DATA.md`
- Modify: `scripts/testlab-acceptance.ps1`

- [ ] **Step 1: Add the new UI command to the documented acceptance sequence**

The sequence must run root Go tests, nested Smart Agent tests, worker smoke, main UI acceptance, Smart Agent API/UI smoke, and testlab acceptance without real external credentials.

- [ ] **Step 2: Run the complete phase-1 sequence**

Run: `go test -buildvcs=false ./...`

Run: `Push-Location HDU-Smart-Course-Agent; go test -buildvcs=false ./...; Pop-Location`

Run: `node scripts/scheduler-worker-smoke.js`

Run: `node scripts/main-ui-acceptance.js`

Run: `node scripts/smart-agent-ui-smoke.js`

- [ ] **Step 3: Review the diff and leave later phases explicitly pending**

Do not claim candidate-flow, release, or Smart Agent safety requirements are complete until their own tests and browser interactions pass.
