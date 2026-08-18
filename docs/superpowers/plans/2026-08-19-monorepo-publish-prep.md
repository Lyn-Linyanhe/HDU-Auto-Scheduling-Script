# Monorepo Publication Preparation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (recommended) to implement this plan task-by-task. Steps use checkbox (\`- [ ]\`) syntax for tracking.

**Goal:** 将排课助手与 Smart Agent 整理为同一个可安全上传的 GitHub monorepo，并让文档、发布包和本地验收入口准确反映这一结构。

**Architecture:** 根目录继续作为 \`HDU-Auto-Scheduling-Script\` 主模块，\`HDU-Smart-Course-Agent/\` 保留独立 Go module，两个程序由根发布脚本构建到同一个 release 包。\`HDU-KillCourse-main\` 继续作为外部依赖，不复制源码；账号、课表快照、运行配置和构建产物由 \`.gitignore\` 排除。

**Tech Stack:** Git、PowerShell、Go modules、Node.js smoke scripts、Markdown。

**Spec:** 用户确认的单仓库方案：保留一个 \`HDU-Auto-Scheduling-Script\` GitHub 仓库，Smart Agent 作为同一产品的独立组件。

## Global Constraints

- 不执行 \`git push\`，除非用户在本次整理完成后明确授权。
- 不修改或删除用户已有工作区改动。
- 不提交账号、密码、Cookie、课程数据、课表快照、执行配置、日志或 \`.exe\` 构建产物。
- 不把外部 \`HDU-KillCourse-main\` 目录复制到本仓库。
- Smart Agent 默认刷新间隔保持 60 秒，配置范围保持 10–7200 秒。

---

### Task 1: Lock the Git publication boundary

**Files:**
- Modify: \`.gitignore\`
- Test: Git index/status inspection commands

**Interfaces:**
- Produces ignore rules for target/current timetable exports, backup files, temporary executable files, and Smart Agent runtime data.

- [x] **Step 1: Record the current dirty-worktree boundary**

Run:

\`\`\`powershell
git status --short
git ls-files --others --exclude-standard
\`\`\`

Expected: existing source/document changes remain visible; personal data and runtime artifacts are candidates for exclusion.

- [x] **Step 2: Extend \`.gitignore\` with explicit local-data patterns**

Add patterns for \`/hdu-current-timetable*.json\`, \`/hdu-target-timetable*.json\`, \`/target-schedule.json\`, \`/course.json.bak-*\`, and \`*.exe~\`, plus the equivalent Smart Agent workspace paths where needed. Keep source fixtures and documentation trackable.

- [x] **Step 3: Verify ignore behavior without staging files**

Run:

\`\`\`powershell
git check-ignore -v hdu-current-timetable.json course.json.bak-20260818-190355 HDU-Auto-Scheduling-Script.exe~
git check-ignore -v target-schedule.json hdu-target-timetable-2026-08-18.json
git status --short
\`\`\`

Expected: all listed local artifacts are ignored; source files under \`HDU-Smart-Course-Agent/\` remain visible as untracked source files.

### Task 2: Make the monorepo structure discoverable

**Files:**
- Modify: \`README.md\`
- Modify: \`HDU-Smart-Course-Agent/README.md\`

**Interfaces:**
- Root README explains that the repository contains two executable components and one external KillCourse dependency.
- Smart Agent README explains its location inside the monorepo and the nested-module test command.

- [x] **Step 1: Add a repository layout section to the root README**

Document \`main.go\`/root scheduler, \`HDU-Smart-Course-Agent/\`, \`school/\`, \`scripts/\`, and \`HDU-KillCourse-main\` as an external directory. State that users clone one repository and release builds produce both executables.

- [x] **Step 2: Complete the release-file documentation**

List \`smart-agent/README.md\` and \`smart-agent/docs/EXECUTION_LOG_SCHEMA.md\` alongside the Smart Agent executable, matching \`scripts/build-release.ps1\` and its manifest.

- [x] **Step 3: Document both Go module test commands**

Keep the root \`go test -buildvcs=false ./...\` command and add a separate \`Push-Location HDU-Smart-Course-Agent; go test -buildvcs=false ./...; Pop-Location\` command, explaining that the nested module is intentionally independent.

- [x] **Step 4: Clarify Smart Agent's monorepo input discovery**

State that the Smart Agent can use the sibling/root scheduler directory and that \`HDU-KillCourse-main\` is configured as an external dependency rather than a second repository upload target.

### Task 3: Include the refresh contract in the standard acceptance path

**Files:**
- Modify: \`scripts/testlab-acceptance.ps1\`
- Test: \`scripts/smart-agent-refresh-contract-test.js\`

**Interfaces:**
- \`testlab-acceptance.ps1\` runs the existing static refresh contract after Smart Agent UI smoke succeeds.
- A non-zero contract result fails the local acceptance run.

- [x] **Step 1: Run the existing contract test as a baseline**

Run:

\`\`\`powershell
node scripts\\smart-agent-refresh-contract-test.js
\`\`\`

Expected: \`Smart Agent refresh contract passed.\`

- [x] **Step 2: Add the contract invocation to TestLab**

Call the Node script immediately after \`scripts\\smart-agent-ui-smoke.js\` and throw when \`$LASTEXITCODE\` is non-zero, matching the neighboring acceptance checks.

- [x] **Step 3: Run the focused acceptance checks**

Run:

\`\`\`powershell
node scripts\\smart-agent-refresh-contract-test.js
node scripts\\smart-agent-ui-smoke.js
\`\`\`

Expected: both commands pass without external school access or real credentials.

### Task 4: Verify the publication-ready tree

**Files:**
- No source changes; inspect \`scripts/build-release.ps1\`, \`README.md\`, and \`.gitignore\`.

- [x] **Step 1: Run repository and nested-module tests**

Run:

\`\`\`powershell
go test -buildvcs=false ./...
Push-Location HDU-Smart-Course-Agent
go test -buildvcs=false ./...
Pop-Location
\`\`\`

Expected: both modules pass independently.

- [x] **Step 2: Build the release package without publishing it**

Run:

\`\`\`powershell
powershell -ExecutionPolicy Bypass -File scripts\\build-release.ps1 -NoZip
\`\`\`

Expected: one release directory contains the main executable, Smart Agent executable, both Smart Agent Markdown files, and \`smart-agent/docs/EXECUTION_LOG_SCHEMA.md\`.

- [x] **Step 3: Inspect the final Git diff and untracked boundary**

Run:

\`\`\`powershell
git diff --check
git status --short
git diff --stat
\`\`\`

Expected: only the intended ignore/documentation/acceptance changes are part of this preparation; user data and build artifacts are not candidates for upload.

- [x] **Step 4: Stop before commit/push and report the exact remote**

Report that the local \`origin\` currently points to \`https://github.com/lyh-123283/HDU-Auto-Scheduling-Script.git\`, while the user-provided public URL is \`https://github.com/Lyn-Linyanhe/HDU-Auto-Scheduling-Script\`. Ask for explicit authorization before changing the remote, committing, or pushing.
