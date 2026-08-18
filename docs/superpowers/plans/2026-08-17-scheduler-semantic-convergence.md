# Scheduler Semantic Convergence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify the main scheduler around exact teaching-class locks, remove the user-visible required-course and teacher-soft-preference concepts, and prove the result with focused tests and browser acceptance.

**Architecture:** Keep `selectedGroups[groupId].lockedItemId` as the single strong course constraint. Move legacy `requiredCourses` handling into a pure one-time migration helper that only auto-locks a unique teaching-class match; ambiguous legacy values become explicit warnings. Keep the existing scheduler search and candidate worker, but remove required-course injection and preferred-teacher scoring.

**Tech Stack:** Existing browser JavaScript, Web Worker scheduler, Node.js smoke/CDP acceptance, localStorage migration, existing Go and PowerShell test harnesses.

**Spec:** `docs/superpowers/specs/2026-08-17-scheduler-semantic-convergence-design.md`

## Global Constraints

- Do not treat an unverified course-name match as a specific teaching-class lock.
- Unlocked selected courses must remain optional and removable in generated candidates.
- Personal-schedule and class-schedule imports must continue to lock imported teaching classes.
- Keep hard time limits, exclusion teachers, forced-together rules, teacher-consistency rules, and free-day-first ordering.
- Do not add real credentials, live教务 calls, automatic KillCourse execution, or real course-selection side effects to tests.
- Preserve unrelated dirty-worktree changes and do not reset or discard existing files.

---

### Task 1: Establish the semantic regression tests

**Files:**
- Modify: `scripts/scheduler-worker-smoke.js`
- Modify: `shared.js`

**Interfaces:**
- `HDU.migrateLegacyCourseLocks(courses, legacyText)` returns `{ matches, unresolved }` where `matches` contains exact course objects and `unresolved` contains original tokens.
- `HDU.evaluateSolution(items, constraints)` remains a pure candidate validator and no longer reads `requiredCourses` or `preferredTeachers`.

- [x] **Step 1: Add a failing migration test**

Add fixtures with two teaching classes under one course name, one unique teaching-class code, and one missing token. Assert only the unique teaching-class code is returned in `matches`; the ambiguous name and missing token are returned in `unresolved`.

- [x] **Step 2: Add failing candidate semantic tests**

Add a generated-candidate assertion that an exact `lockedItemId` remains in every result while a selected group with no lock can be omitted. Add a score assertion showing the same candidate has the same score when a legacy `preferredTeachers` value is present in the input state.

- [x] **Step 3: Run the focused smoke test and verify RED**

Run: `node scripts/scheduler-worker-smoke.js`

Expected: the new migration export and the old preferred-teacher behavior cause focused assertions to fail for the expected missing behavior.

- [x] **Step 4: Export the smallest pure APIs needed by the tests**

Expose `migrateLegacyCourseLocks` and `evaluateSolution` from `shared.js` without changing behavior yet. Re-run the smoke test and confirm failures now identify behavior rather than missing test setup.

### Task 2: Implement state migration and exact-lock semantics

**Files:**
- Modify: `shared.js`
- Modify: `scheduler.js`
- Modify: `scheduler-worker.js`
- Modify: `scripts/scheduler-worker-smoke.js`

**Interfaces:**
- `HDU.loadState()` moves old `requiredCourses` into an internal legacy migration field and does not return it as an active scheduler constraint.
- `HDU.migrateLegacyCourseLocks()` resolves exact item identifiers first, then accepts a result only when the legacy token maps to one unique item.
- `schedulerState()` sends only active constraints and selected groups to the worker.

- [x] **Step 1: Implement unique legacy matching**

Match exact `id`, `displayCode`, `sectionName`, and `rawCourseCode` values before falling back to the existing course-group resolver. Deduplicate by item ID and classify zero or multiple matches as unresolved.

- [x] **Step 2: Remove active required-course state from the scheduler**

Remove `requiredCourses` from default active constraints, `constraintsFromState`, candidate-group construction, required hit explanations, and required-specific diagnostics. Keep only the one-time legacy reader and warning state.

- [x] **Step 3: Apply migrated matches as selected and locked items**

After courses load, add each unique migration match to its group and set `lockedItemId` to that item ID. Render unresolved migration warnings without reintroducing a required-course input.

- [x] **Step 4: Remove preferred-teacher scoring**

Stop reading `preferredTeachers` in active constraints and remove its score adjustment. Keep the fixed time/credit score and free-day ordering unchanged.

- [x] **Step 5: Run focused tests and verify GREEN**

Run: `node scripts/scheduler-worker-smoke.js`

Expected: migration, locked-course, optional-course, scheme-rule, and score assertions pass.

### Task 3: Replace the required-course UI with lock actions

**Files:**
- Modify: `scheduler.html`
- Modify: `scheduler.js`
- Modify: `styles.css`

**Interfaces:**
- The course-level constraint block exposes stable IDs for locked teaching-class rows, quick lock actions, all-course lock search, and legacy migration warnings.
- Every all-course search action identifies one concrete course item and invokes the same add-then-lock path used by the selected-list lock button.

- [x] **Step 1: Add failing browser assertions**

Extend `scripts/main-ui-acceptance.js` to fail when required-course selectors or preferred-teacher labels remain, and to require the locked-course block, direct lock search action, and course-level DOM order.

- [x] **Step 2: Replace the HTML controls**

Remove the hidden required textarea and required-only boxes. Add locked-course summary, selected-course quick-lock controls, all-course search results, and a live migration warning region. Remove the preferred-teacher textarea.

- [x] **Step 3: Replace the scheduler render and event flow**

Render concrete teaching-class rows, add a direct add-and-lock handler, keep normal course-list add as unlocked, and update selected-list/status copy to use lock terminology. Remove required token renderers and handlers.

- [x] **Step 4: Add focused CSS for the new controls**

Reuse existing quick-constraint and search styles, add only the minimum row/warning styles required for readable desktop and mobile layout, and keep fixed control dimensions stable.

- [x] **Step 5: Run browser acceptance and verify GREEN**

Run: `node scripts/main-ui-acceptance.js`

Expected: no required/preference selectors remain; course-only and course-plus-personal scenarios pass on desktop and mobile viewports.

### Task 4: Protect scheme rules and update explanations

**Files:**
- Modify: `scripts/scheduler-worker-smoke.js`
- Modify: `scheduler.js`
- Modify: `shared.js`
- Modify: `docs/USER_GUIDE.md`

**Interfaces:**
- `pairRules` continues to require both courses to be jointly present or absent.
- `sameTeacherRules` continues to reject selected pairs without a shared teacher.
- Result explanations name locked courses and fixed ranking metrics without “必选” or “偏好教师” terminology.

- [x] **Step 1: Add failing rule behavior assertions**

Test pair rule single-side rejection, pair rule both-side acceptance, teacher-consistency mismatch rejection, and teacher-consistency match acceptance using the existing smoke harness fixtures.

- [x] **Step 2: Update diagnostics and result copy**

Replace mixed “必选/锁定” recommendations with “锁定课程/已选课程” wording. Replace advice to use preferred teachers with advice to adjust exclusion or teacher-consistency rules.

- [x] **Step 3: Run the focused worker test**

Run: `node scripts/scheduler-worker-smoke.js`

Expected: all scheme-rule and explanation assertions pass.

### Task 5: Add structural UI acceptance and document external boundaries

**Files:**
- Modify: `scripts/main-ui-acceptance.js`
- Modify: `README.md`
- Modify: `docs/TEST_DATA.md`
- Modify: `项目待办.md`

**Interfaces:**
- Main UI acceptance reports the presence/order of lock controls, absence of removed concepts, base schedule auto-locking, and no horizontal overflow.
- Documentation distinguishes local snapshot capacity data from external real-time execution.

- [x] **Step 1: Add structure assertions**

Assert the locked-course summary precedes the quick-lock/search controls, search results operate on concrete teaching-class IDs, and the controls remain within the viewport at 1440px and 390px.

- [x] **Step 2: Update user documentation**

Document exact teaching-class locking, optional unlocked selections, legacy-state migration behavior, fixed ranking metrics, and Smart Agent's manual external execution boundary.

- [x] **Step 3: Mark the resolved and externally bounded items**

Update the progress audit for items 18, 22, and 27 after tests pass. Keep items 16, 36, and 37 explicitly bounded by course-data quality or external real-time systems.

### Task 6: Full verification and release audit

**Files:**
- Modify only if verification exposes a regression.

- [x] **Step 1: Run unit and worker checks**

Passed: `go test -buildvcs=false ./...`

Passed: `Push-Location HDU-Smart-Course-Agent; go test -buildvcs=false ./...; Pop-Location`

Passed: `node scripts/scheduler-worker-smoke.js`

- [x] **Step 2: Run browser and testlab acceptance**

Passed: `node scripts/main-ui-acceptance.js` (desktop and mobile viewports, including concrete teaching-class locking and legacy-state migration).

Passed: `powershell -ExecutionPolicy Bypass -File scripts/smart-agent-e2e.ps1`

Passed: `node scripts/smart-agent-ui-smoke.js` (desktop `1440x1000` and mobile `390x844`).

Passed: `powershell -ExecutionPolicy Bypass -File scripts/testlab-acceptance.ps1`

- [x] **Step 3: Build and smoke the release**

Built and verified `release/HDU-Auto-Scheduling-Script-v0.5.3-local-20260817-221348/` with:

`powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 -Version 0.5.3-local-20260817-221348`

`powershell -ExecutionPolicy Bypass -File release/HDU-Auto-Scheduling-Script-v0.5.3-local-20260817-221348/scripts/release-main-smoke.ps1 -PackageDir release/HDU-Auto-Scheduling-Script-v0.5.3-local-20260817-221348`

`powershell -ExecutionPolicy Bypass -File release/HDU-Auto-Scheduling-Script-v0.5.3-local-20260817-221348/scripts/smart-agent-e2e.ps1`

`node release/HDU-Auto-Scheduling-Script-v0.5.3-local-20260817-221348/scripts/smart-agent-ui-smoke.js`

- [x] **Step 4: Review evidence and leave the goal active if external boundaries remain**

All in-repository requirements are verified and the audit records the remaining external boundaries. The local tests do not claim real-time教务 data or automatic KillCourse execution; the goal remains bounded by those external systems.
