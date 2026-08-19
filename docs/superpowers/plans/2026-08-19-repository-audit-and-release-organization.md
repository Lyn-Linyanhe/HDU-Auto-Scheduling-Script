# Repository Audit and Release Organization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (\`- [ ]\`) syntax for tracking.

**Goal:** 将当前 monorepo 的源码、个人运行数据和发布制品边界整理清楚，补强隐私忽略规则，并准备可核验的 \`v0.5.3\` GitHub Release 包。

**Architecture:** 保持根目录主排课助手与 \`HDU-Smart-Course-Agent/\` 独立 Go module 的 monorepo 结构。Git 只保存源码、文档、测试样例和构建脚本；\`release/\`、\`dist/\` 与运行数据仍留在本地，正式 ZIP 通过 GitHub Release asset 分发。

**Tech Stack:** Go modules, PowerShell release scripts, static HTML/JavaScript UI, Git/GitHub Releases.

**Spec:** \`docs/superpowers/specs/2026-08-19-repository-audit-and-release-organization.md\`

## Global Constraints

- Do not add real account credentials, cookies, personal timetable data, browser profiles, executables, ZIP files, or logs to Git.
- Do not copy \`HDU-KillCourse-main\` into this repository.
- Do not delete historical release outputs or user data in this pass; generated outputs may be rebuilt in their exact output directory by the existing release script.
- Do not create or modify a GitHub Release in this pass; prepare and verify the asset first.
- Preserve the existing Smart Agent API contract in this pass; report the sensitive \`generatedConfig\` response as a follow-up security change.

---

### Task 1: Document Repository and Release Boundaries

**Files:**
- Create: \`docs/REPOSITORY_LAYOUT.md\`
- Modify: \`README.md\`

**Interfaces:**
- Consumes: the public/local/release boundary in \`docs/superpowers/specs/2026-08-19-repository-audit-and-release-organization.md\`.
- Produces: user-facing documentation that explains where source files, local data, build output, and GitHub Release assets belong.

- [ ] **Step 1: Write the repository layout document**

Create \`docs/REPOSITORY_LAYOUT.md\` with these sections and exact rules:

~~~markdown
# Repository Layout and Publishing Boundary

## Track In Git

Source, tests, deterministic \`testdata/\`, documentation, build scripts, \`VERSION\`, and module files.

## Keep Local

Course exports, personal/current/target timetable snapshots, login configuration, cookies, logs, execution plans, browser profiles, \`dist/\`, \`release/\`, temporary folders, executables, ZIP files, and private notes.

## External Dependency

\`HDU-KillCourse-main\` is configured as an external directory. It is not part of this repository.

## Release Flow

1. Run \`powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1\`.
2. Inspect \`release/HDU-Auto-Scheduling-Script-v<version>/manifest.json\`.
3. Run the release main smoke and Smart Agent checks from the generated package.
4. Upload only the generated ZIP as a GitHub Release asset for the matching tag.

## Version Check

The tag, \`VERSION\`, package directory, ZIP name, and manifest version must match.
~~~

- [ ] **Step 2: Link the boundary from the root README**

Add a short section near the repository structure/release instructions in \`README.md\`:

~~~markdown
### 仓库文件边界

源码仓库只提交源码、文档、测试样例和构建脚本。个人课表、登录配置、Cookie、运行日志、浏览器目录、exe、ZIP 和 \`release/\` 均为本地产物；可分发 ZIP 应作为 GitHub Release asset 上传。详细规则见 [docs/REPOSITORY_LAYOUT.md](docs/REPOSITORY_LAYOUT.md)。
~~~

- [ ] **Step 3: Verify documentation references**

Run:

~~~powershell
rg -n "REPOSITORY_LAYOUT|GitHub Release|HDU-KillCourse-main|release/" README.md docs/REPOSITORY_LAYOUT.md
~~~

Expected: the new boundary document is linked, the external dependency is named, and the Release asset flow is described.

### Task 2: Harden Local-Artifact Ignore Rules

**Files:**
- Modify: \`.gitignore\`

**Interfaces:**
- Consumes: existing source paths and local output paths.
- Produces: ignore rules for accidental root-level package and browser artifacts.

- [ ] **Step 1: Add narrow privacy/build rules**

Append these rules to the corresponding existing sections in \`.gitignore\`:

~~~gitignore
# Generated packages and local backups
*.zip
*.bak-*

# Browser profile databases created by UI acceptance runs
*.db
*.db-wal
*.db-shm
~~~

Do not add a wildcard for \`*.json\` or \`*.md\`; deterministic fixtures and public documentation must remain trackable.

- [ ] **Step 2: Verify representative paths**

Run:

~~~powershell
git check-ignore -v release/HDU-Auto-Scheduling-Script-v0.5.3.zip unexpected-local-export.zip course.json.bak-20260819 .tmp-edge-profile/Cookies.db
~~~

Expected: every path is ignored and the output points to the new or existing rule.

- [ ] **Step 3: Verify tracked files are unchanged**

Run:

~~~powershell
git diff --check
git ls-files --error-unmatch testdata/course.sample.json testdata/personal-schedule.sample.json README.md
git status --short
~~~

Expected: no whitespace errors, all three public files remain tracked, and only the intended \`.gitignore\` change is listed.

### Task 3: Rebuild and Verify the Current Release Package

**Files:**
- Modify: generated, ignored files under \`release/\` and \`dist/\` only through \`scripts/build-release.ps1\`.
- Modify: \`scripts/release-main-smoke.ps1\` to accept a non-default port when another local main assistant is running.
- Test: \`scripts/release-main-smoke.ps1\`, \`scripts/smart-agent-e2e.ps1\`, \`scripts/smart-agent-ui-smoke.js\`.

**Interfaces:**
- Consumes: current \`master\`, \`VERSION\`, source modules, and deterministic \`testdata/\`.
- Produces: \`release/HDU-Auto-Scheduling-Script-v0.5.3.zip\` and a manifest that matches the package contents.

- [ ] **Step 1: Build from the current version**

Run:

~~~powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 -Version 0.5.3
~~~

Expected: the script runs both module test suites, creates the main and Smart Agent executables, and writes the \`v0.5.3\` directory and ZIP under \`release/\`.

- [ ] **Step 2: Verify the manifest and package contents**

Run:

~~~powershell
$package = "release/HDU-Auto-Scheduling-Script-v0.5.3"
$manifest = Get-Content "$package/manifest.json" -Raw | ConvertFrom-Json
$manifest.version
$manifest.files | ForEach-Object { Test-Path (Join-Path $package $_) }
~~~

Expected: version is \`0.5.3\` and every manifest entry prints \`True\`; no credential, personal timetable, or browser database appears in the manifest.

- [ ] **Step 3: Run release checks**

Run:

~~~powershell
$package = "release/HDU-Auto-Scheduling-Script-v0.5.3"
powershell -ExecutionPolicy Bypass -File "$package/scripts/release-main-smoke.ps1" -PackageDir $package -Port 6903
powershell -ExecutionPolicy Bypass -File "$package/scripts/smart-agent-e2e.ps1"
node "$package/scripts/smart-agent-ui-smoke.js"
~~~

Expected: all checks pass without touching the real \`HDU-KillCourse/config.json\` or executing a real course selection operation.

### Task 4: Audit Gate and Handoff

**Files:**
- Inspect: \`git status\`, \`git diff\`, \`.gitignore\`, \`README.md\`, \`docs/REPOSITORY_LAYOUT.md\`.
- Inspect: \`HDU-Smart-Course-Agent/main.go:1151-1188\` for the recorded sensitive-response finding.

**Interfaces:**
- Consumes: the organized documentation, ignore rules, and verified local package.
- Produces: an evidence-backed audit report and a clean commit ready for review; GitHub Release publication remains a separate explicit action.

- [ ] **Step 1: Run the repository safety audit**

Run:

~~~powershell
git status --short --branch
git diff --check
git diff --stat
git ls-files | rg "(^|/)(course|personal-schedule|target-schedule|hdu-current|login-config|execution-|agent-settings|run-killcourse)|\.exe$|\.zip$|\.db(-wal|-shm)?$"
~~~

Expected: no personal runtime data, executable, ZIP, or browser database is tracked.

- [ ] **Step 2: Run the code-review gate**

Review the final diff for accidental data-boundary changes, stale version text, and documentation paths. Treat the unredacted \`generatedConfig\` response as a follow-up security item rather than silently ignoring it.

- [ ] **Step 3: Commit the organization changes**

~~~powershell
git add .gitignore README.md docs/REPOSITORY_LAYOUT.md docs/superpowers/specs/2026-08-19-repository-audit-and-release-organization.md docs/superpowers/plans/2026-08-19-repository-audit-and-release-organization.md
git commit -m "docs: clarify repository and release boundaries"
~~~

- [ ] **Step 4: Confirm handoff state**

Run:

~~~powershell
git status --short --branch
git log -1 --oneline --decorate
~~~

Expected: the organization commit is present, the working tree is clean, and the next independent action is uploading the freshly verified ZIP to a new \`v0.5.3\` GitHub Release.
