# Repository Presentation and Layout Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变用户入口、公开 URL、Go module 边界和 Release 内容的前提下，收敛 GitHub 根目录、归档历史说明、补充文档导航，并建立可自动核验的仓库布局。

**Architecture:** 保持根 Go module 作为统一主程序，保持 `HDU-Smart-Course-Agent/` 为 monorepo 内的独立 Go module。将主程序静态资源从仓库根目录移动到 `web/`，由 `serveStatic` 把现有根 URL 映射到新的嵌入目录；保留 `cmd/course-exporter/web/`，因为统一主程序仍通过 `/exporter/` 使用这组资源。

**Tech Stack:** Go 1.23, `embed.FS`, static HTML/CSS/JavaScript, Node.js smoke tests, PowerShell, GitHub Actions.

**Spec:** `docs/REPOSITORY_LAYOUT.md`

## Global Constraints

- Keep the repository as one monorepo; do not split `HDU-Smart-Course-Agent/` into another repository.
- Keep both existing Go modules and their `go 1.23` declarations unchanged.
- Preserve the public routes `/`, `/bootstrap.html`, `/scheduler.html`, `/exporter/`, `/styles.css`, `/shared.js`, `/bootstrap.js`, `/scheduler.js`, and `/scheduler-worker.js`.
- Preserve `/exporter/style.css` and `/exporter/main.js`; `cmd/course-exporter/web/` is a runtime dependency of the unified main executable.
- Do not rename or remove `cmd/course-exporter` in this pass; document its standalone Go command as a compatibility/development entry.
- Do not introduce npm, a JavaScript bundler, generated frontend output, or a new runtime dependency.
- Do not rewrite dated files under `docs/superpowers/plans/` or `docs/superpowers/specs/`; they are historical engineering records and may contain paths that were correct when written.
- Do not track credentials, cookies, personal timetable data, browser profiles, executables, ZIP files, databases, logs, or generated files under `dist/` and `release/`.
- Do not copy the external `HDU-KillCourse-main` directory into this repository or package its real local configuration.
- Do not change `VERSION`, create a tag, publish a GitHub Release, or alter the release manifest file list in this pass.
- License selection and a public security-reporting address require a maintainer decision and are outside this layout-only plan.

## Target Tracked Layout

```text
.
|-- .github/
|   `-- workflows/ci.yml
|-- cmd/
|   |-- README.md
|   |-- course-exporter/
|   `-- hdu-testlab/
|-- docs/
|   |-- README.md
|   |-- archive/2026-07-smart-agent-project-brief.md
|   |-- COURSE_SCHEMA.md
|   |-- REPOSITORY_LAYOUT.md
|   |-- TEST_DATA.md
|   |-- USER_GUIDE.md
|   `-- superpowers/
|-- HDU-Smart-Course-Agent/
|-- school/
|-- scripts/
|-- testdata/
|-- web/
|   |-- index.html
|   |-- bootstrap.html
|   |-- bootstrap.js
|   |-- scheduler.html
|   |-- scheduler.js
|   |-- scheduler-worker.js
|   |-- shared.js
|   `-- styles.css
|-- .gitignore
|-- go.mod
|-- main.go
|-- main_test.go
|-- README.md
`-- VERSION
```

## Execution Preflight

- [ ] **Step 1: Confirm the approved plan is the only pending change**

Run:

```powershell
git status --short --branch
git diff --check
```

Expected: this plan document is the only untracked or modified file. Stop and review any unrelated change instead of including it in the cleanup.

- [ ] **Step 2: Record the approved plan before implementation**

```powershell
git add docs/superpowers/plans/2026-08-20-repository-presentation-and-layout-cleanup.md
git commit -m "docs: plan repository layout cleanup"
```

---

### Task 0: Make Smart Agent UI Smoke Hermetic

**Files:**
- Modify: `scripts/smart-agent-ui-smoke.js:197-210`
- Modify: `docs/superpowers/plans/2026-08-20-repository-presentation-and-layout-cleanup.md` when a clean-worktree RED/GREEN command reveals a missing prerequisite.

**Interfaces:**
- Consumes: optional `HDU_COURSE_FIXTURE`, source fixture `testdata/course.sample.json`, release fixture `samples/course.sample.json`, and the existing temporary Smart Agent UI workspace.
- Produces: a UI smoke test that runs in a clean source checkout, consumes testlab's exported fixture when provided, and remains runnable from a release package.

- [ ] **Step 1: Reproduce the clean-checkout failure**

Run from an isolated worktree that has no ignored root `course.json`:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/testlab-acceptance.ps1
```

Expected before the fix: FAIL at `smart-agent-ui-smoke.js` with `course.json or samples/course.sample.json is required`. Earlier mock export, Worker smoke, main UI, and Smart Agent E2E stages pass, proving the failure is fixture discovery rather than fixture generation.

- [ ] **Step 2: Use deterministic fixture precedence**

Replace the existing `sourceCourse` / `sampleCourse` selection in `prepareTempWorkspace()` with:

```javascript
  const courseCandidates = [
    process.env.HDU_COURSE_FIXTURE
      ? path.resolve(process.env.HDU_COURSE_FIXTURE)
      : '',
    path.join(root, 'course.json'),
    path.join(root, 'testdata', 'course.sample.json'),
    path.join(root, 'samples', 'course.sample.json'),
  ].filter(Boolean);
  const source = courseCandidates.find((candidate) => fs.existsSync(candidate));
  if (!source) {
    throw new Error(
      'HDU_COURSE_FIXTURE, course.json, testdata/course.sample.json, or samples/course.sample.json is required for Smart Agent UI smoke test.',
    );
  }
```

Do not alter the fixture payload normalization, temporary workspace paths, browser checks, or cleanup behavior.

- [ ] **Step 3: Verify the source and testlab paths turn green**

Run:

```powershell
$previousFixture = [Environment]::GetEnvironmentVariable("HDU_COURSE_FIXTURE", "Process")
$previousAgentExe = [Environment]::GetEnvironmentVariable("HDU_SMART_AGENT_EXE", "Process")
$sourceSmokeExe = Join-Path $env:TEMP "HDU-Smart-Course-Agent-source-smoke.exe"
Push-Location HDU-Smart-Course-Agent
try { go build -buildvcs=false -o $sourceSmokeExe . } finally { Pop-Location }
Remove-Item Env:HDU_COURSE_FIXTURE -ErrorAction SilentlyContinue
$env:HDU_SMART_AGENT_EXE = $sourceSmokeExe
try {
  node scripts/smart-agent-ui-smoke.js
  powershell -ExecutionPolicy Bypass -File scripts/testlab-acceptance.ps1
} finally {
  if ($null -eq $previousFixture) {
    Remove-Item Env:HDU_COURSE_FIXTURE -ErrorAction SilentlyContinue
  } else {
    $env:HDU_COURSE_FIXTURE = $previousFixture
  }
  if ($null -eq $previousAgentExe) {
    Remove-Item Env:HDU_SMART_AGENT_EXE -ErrorAction SilentlyContinue
  } else {
    $env:HDU_SMART_AGENT_EXE = $previousAgentExe
  }
  if (Test-Path -LiteralPath $sourceSmokeExe) {
    Remove-Item -LiteralPath $sourceSmokeExe -Force
  }
}
```

Expected: the direct source smoke uses `testdata/course.sample.json`; the full testlab acceptance uses its exported `HDU_COURSE_FIXTURE`; both pass.

- [ ] **Step 4: Commit the hermetic smoke fix**

```powershell
git add scripts/smart-agent-ui-smoke.js docs/superpowers/plans/2026-08-20-repository-presentation-and-layout-cleanup.md
git commit -m "test: make Smart Agent UI smoke hermetic"
```

### Task 1: Lock the Existing Public Static-Asset Contract

**Files:**
- Modify: `main_test.go`

**Interfaces:**
- Consumes: `serveStatic(http.ResponseWriter, *http.Request)` and the current root-level embedded assets.
- Produces: `TestServeStaticPreservesPublicURLs`, a characterization test that must pass both before and after the physical file move.

- [ ] **Step 1: Add the public-route characterization test**

Add this test after `TestServeExporterStatic` in `main_test.go`:

```go
func TestServeStaticPreservesPublicURLs(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
		marker      string
	}{
		{"/", "text/html; charset=utf-8", "<title>HDU 课表自动化编排助手</title>"},
		{"/bootstrap.html", "text/html; charset=utf-8", "<title>HDU 模拟排课助手 - 导入</title>"},
		{"/scheduler.html", "text/html; charset=utf-8", "<title>HDU 课表自动化编排助手</title>"},
		{"/styles.css", "text/css; charset=utf-8", ".bootstrap-layout"},
		{"/shared.js", "application/javascript; charset=utf-8", "globalThis.HDU"},
		{"/bootstrap.js", "application/javascript; charset=utf-8", "/api/bootstrap/import"},
		{"/scheduler.js", "application/javascript; charset=utf-8", "/api/export/timetable"},
		{"/scheduler-worker.js", "application/javascript; charset=utf-8", "self.onmessage"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			serveStatic(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("serveStatic(%q) status = %d, want %d", tt.path, rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.contentType {
				t.Fatalf("serveStatic(%q) content type = %q, want %q", tt.path, got, tt.contentType)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.marker) {
				t.Fatalf("serveStatic(%q) body does not contain %q", tt.path, tt.marker)
			}
		})
	}
}
```

- [ ] **Step 2: Run the characterization test before moving files**

Run:

```powershell
go test -buildvcs=false ./... -run "TestServeStaticPreservesPublicURLs|TestServeExporterStatic"
```

Expected: PASS. This test records the current externally visible behavior; it is intentionally green before the refactor.

- [ ] **Step 3: Commit the contract test**

```powershell
git add main_test.go
git commit -m "test: lock public web asset routes"
```

### Task 2: Move the Main Frontend into `web/`

**Files:**
- Create by move: `web/index.html`
- Create by move: `web/bootstrap.html`
- Create by move: `web/bootstrap.js`
- Create by move: `web/scheduler.html`
- Create by move: `web/scheduler.js`
- Create by move: `web/scheduler-worker.js`
- Create by move: `web/shared.js`
- Create by move: `web/styles.css`
- Modify: `main.go:22`, `main.go:138-153`
- Modify: `main_test.go:219-230`
- Modify: `scripts/scheduler-worker-smoke.js:5-12`

**Interfaces:**
- Consumes: the public-route contract from Task 1 and the existing relative Worker dependency `scheduler-worker.js -> shared.js`.
- Produces: the same HTTP asset paths backed by embedded files under `web/`; repository-root frontend files no longer exist.

- [ ] **Step 1: Move the eight tracked frontend files**

Run:

```powershell
New-Item -ItemType Directory -Path web -ErrorAction Stop | Out-Null
git mv index.html web/index.html
git mv bootstrap.html web/bootstrap.html
git mv bootstrap.js web/bootstrap.js
git mv scheduler.html web/scheduler.html
git mv scheduler.js web/scheduler.js
git mv scheduler-worker.js web/scheduler-worker.js
git mv shared.js web/shared.js
git mv styles.css web/styles.css
```

Expected: Git records eight renames; the file contents and browser-visible URLs are unchanged.

- [ ] **Step 2: Point the embedded filesystem at `web/`**

Change the embed directive in `main.go`:

```go
//go:embed web/* cmd/course-exporter/web/*
var webFS embed.FS
```

Change only the read path inside `serveStatic`:

```go
data, err := webFS.ReadFile(path.Join("web", name))
```

Do not change `serveExporterStatic`; it must continue reading `path.Join("cmd/course-exporter/web", name)`.

- [ ] **Step 3: Update source-level tests and the Worker smoke loader**

In `TestSchedulerExportUsesProjectWriter`, change:

```go
data, err := os.ReadFile("web/scheduler.js")
```

At the top of `scripts/scheduler-worker-smoke.js`, use a separate source root for frontend files:

```javascript
const root = path.resolve(__dirname, '..');
const webRoot = path.join(root, 'web');

function readText(file) {
  return fs.readFileSync(path.join(webRoot, file), 'utf8');
}
```

Keep `courseFixture` rooted at `path.join(root, 'testdata/course.sample.json')`. Keep the browser-relative strings `scheduler-worker.js` and `shared.js` unchanged inside the frontend source.

- [ ] **Step 4: Format and run focused verification**

Run:

```powershell
gofmt -w main.go main_test.go
go test -buildvcs=false ./... -run "TestServeStaticPreservesPublicURLs|TestServeExporterStatic|TestSchedulerExportUsesProjectWriter"
node scripts/scheduler-worker-smoke.js
go build -buildvcs=false -o "$env:TEMP\HDU-layout-check.exe" .
```

Expected: all tests pass, the Worker smoke prints its success result, and the main executable builds with the new embed pattern.

- [ ] **Step 5: Verify root-directory contraction**

Run:

```powershell
$moved = @("index.html", "bootstrap.html", "bootstrap.js", "scheduler.html", "scheduler.js", "scheduler-worker.js", "shared.js", "styles.css")
$remaining = $moved | Where-Object { Test-Path -LiteralPath $_ }
if ($remaining) { throw "Frontend files remain at repository root: $($remaining -join ', ')" }
Get-ChildItem web -File | Sort-Object Name | Select-Object -ExpandProperty Name
```

Expected: no exception; the eight expected files are listed under `web/`.

- [ ] **Step 6: Commit the frontend relocation**

```powershell
git add main.go main_test.go scripts/scheduler-worker-smoke.js web
git commit -m "refactor: group main frontend assets under web"
```

### Task 3: Add Public Documentation Navigation and Archive the Early Brief

**Files:**
- Create: `docs/README.md`
- Create by move: `docs/archive/2026-07-smart-agent-project-brief.md`
- Remove by move: `HDU智能选课执行助手-Codex项目说明.md`
- Modify: `README.md`
- Modify: `docs/REPOSITORY_LAYOUT.md`
- Create: `cmd/README.md`

**Interfaces:**
- Consumes: the target layout produced by Task 2 and the existing source/local/release boundary.
- Produces: one public documentation index, one command index, an explicitly archived project brief, and a root README that describes the actual tree.

- [ ] **Step 1: Move and label the historical Smart Agent brief**

Run:

```powershell
New-Item -ItemType Directory -Path docs\archive -ErrorAction Stop | Out-Null
git mv "HDU智能选课执行助手-Codex项目说明.md" "docs\archive\2026-07-smart-agent-project-brief.md"
```

Replace the first heading and insert the archive notice immediately after it:

```markdown
# HDU 智能选课执行助手早期项目说明（归档）

> 本文记录 2026 年 7 月的早期目标、里程碑和协作约定，保留用于追溯设计背景，不代表当前实现状态。当前使用说明见 [`HDU-Smart-Course-Agent/README.md`](../../HDU-Smart-Course-Agent/README.md) 和 [`HDU-Smart-Course-Agent/SMART_AGENT_QUICKSTART.md`](../../HDU-Smart-Course-Agent/SMART_AGENT_QUICKSTART.md)。
```

Do not update the remaining historical milestone text or its old directory-tree snapshot.

- [ ] **Step 2: Create the documentation index**

Create `docs/README.md` with this content:

```markdown
# Documentation

## User Documentation

- [User Guide](USER_GUIDE.md): installation, startup paths, and normal workflows.
- [Course Schema](COURSE_SCHEMA.md): normalized course payload schema v1.
- [Test Data](TEST_DATA.md): deterministic fixtures and their permitted use.

## Repository Maintenance

- [Repository Layout](REPOSITORY_LAYOUT.md): tracked source, local data, and GitHub Release boundaries.
- [Smart Agent Early Project Brief](archive/2026-07-smart-agent-project-brief.md): archived July 2026 design context.

## Engineering History

`superpowers/specs/` contains dated design records. `superpowers/plans/` contains dated implementation plans. Paths and status statements in those files describe the repository at the time they were written and are not current user documentation.
```

- [ ] **Step 3: Create the command index**

Create `cmd/README.md` with this content:

~~~markdown
# Commands

## `course-exporter`

Compatibility and development entry for running the course exporter on its own. The unified main program reuses `cmd/course-exporter/web/` at `/exporter/`, so the web directory remains a runtime source dependency even though the standalone command is not included as a separate executable in release packages.

Run with:

```powershell
go run ./cmd/course-exporter
```

## `hdu-testlab`

Deterministic loopback-only teaching-system simulator used by acceptance tests. It never contacts HDU endpoints and must continue to reject non-loopback listen addresses.
~~~

- [ ] **Step 4: Update the root README structure table**

Replace the current repository structure table with:

```markdown
| 目录 | 作用 |
| --- | --- |
| 根目录、`main.go` | 主排课助手的 Go 入口和统一 HTTP 服务 |
| `web/` | 主排课助手的内嵌 HTML、CSS、JavaScript 和 Worker |
| `school/` | 教务登录、全校课程和个人课表导出 |
| `HDU-Smart-Course-Agent/` | 独立 Go module，负责把目标课表转换为 KillCourse 执行准备 |
| `cmd/` | 兼容导出器和仅供验收使用的本地 testlab 命令 |
| `scripts/` | 构建、发布包、自检和 UI/API 验收脚本 |
| `docs/` | 用户文档、仓库规范和带日期的工程记录 |
| `testdata/` | 不含真实账号和个人课表的确定性测试样例 |
```

In the local-development section, describe `go run ./cmd/course-exporter` as a compatibility/development entry and state that release users should run the unified executable.

- [ ] **Step 5: Update the repository boundary document**

In `docs/REPOSITORY_LAYOUT.md`:

- add `web/` to tracked frontend source;
- link `docs/README.md` as the documentation index;
- state that `docs/superpowers/` is dated engineering history;
- state that `cmd/course-exporter/web/` is shared by the unified main program and must not be removed as an unused legacy directory;
- keep every existing local-data and Release-asset rule unchanged.

- [ ] **Step 6: Verify current links without rewriting history**

Run:

```powershell
rg -n 'HDU智能选课执行助手-Codex项目说明|根目录、`main.go`、`scheduler.html`' README.md docs/README.md docs/REPOSITORY_LAYOUT.md cmd/README.md
rg -n "docs/README|web/|course-exporter/web|superpowers" README.md docs/README.md docs/REPOSITORY_LAYOUT.md cmd/README.md
git diff --check
```

Expected: the first command returns no current-document references; the second finds the new navigation and boundary text. Matches inside the archived brief or dated plans are acceptable historical content.

- [ ] **Step 7: Commit the documentation organization**

```powershell
git add README.md docs cmd/README.md
git commit -m "docs: organize public repository guidance"
```

### Task 4: Add an Automated Repository-Layout Gate

**Files:**
- Create: `scripts/repository-layout-check.ps1`
- Create: `scripts/repository-layout-check-test.ps1`
- Create: `.github/workflows/ci.yml`
- Modify: `README.md`
- Modify: `docs/superpowers/plans/2026-08-20-repository-presentation-and-layout-cleanup.md` when task review reveals an omitted layout rule.

**Interfaces:**
- Consumes: the target tree from Tasks 2 and 3 and the existing two-module test commands.
- Produces: a local layout check and a GitHub Actions job that detects root-file regressions, tracked private artifacts, broken builds, and missed submodule tests.

- [ ] **Step 1: Create the local layout checker**

Create `scripts/repository-layout-check.ps1` with this content:

```powershell
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$requiredWebFiles = @(
  "web/index.html",
  "web/bootstrap.html",
  "web/bootstrap.js",
  "web/scheduler.html",
  "web/scheduler.js",
  "web/scheduler-worker.js",
  "web/shared.js",
  "web/styles.css"
)
$legacyRootFiles = @(
  "index.html",
  "bootstrap.html",
  "bootstrap.js",
  "scheduler.html",
  "scheduler.js",
  "scheduler-worker.js",
  "shared.js",
  "styles.css",
  "HDU智能选课执行助手-Codex项目说明.md"
)
$privateLeafNames = @(
  "course.json",
  "course-export-diagnosis.json",
  "personal-schedule.json",
  "personal-schedule-live.json",
  "target-schedule.json",
  "login-config.json",
  "live-schedule-sync.json",
  "action-plan.json",
  "agent-settings.json",
  "execution-approval.json",
  "execution-package.json",
  "execution-log.json",
  "fallback-recommendations.json",
  "execution-runbook.md",
  "run-killcourse.bat"
)
$privatePathPatterns = @(
  "(^|/)hdu-(current|target)-timetable[^/]*\.json$",
  "(^|/)(HDU-Smart-Course-Agent|选课脚本)/config\.json$",
  "^(dist|release)/",
  "\.(exe|zip|db|db-wal|db-shm|log|xlsx|xls)$",
  "\.bak-[^/]+$"
)

Push-Location $repoRoot
try {
  $missing = $requiredWebFiles | Where-Object { -not (Test-Path -LiteralPath $_ -PathType Leaf) }
  if ($missing) {
    throw "Missing required web files: $($missing -join ', ')"
  }

  $stale = $legacyRootFiles | Where-Object { Test-Path -LiteralPath $_ }
  if ($stale) {
    throw "Files must not remain at repository root: $($stale -join ', ')"
  }

  $tracked = @(git ls-files)
  if ($LASTEXITCODE -ne 0) {
    throw "git ls-files failed"
  }

  $private = foreach ($trackedPath in $tracked) {
    $leaf = Split-Path -Leaf $trackedPath
    $matchesPrivatePattern = $false
    foreach ($pattern in $privatePathPatterns) {
      if ($trackedPath -match $pattern) {
        $matchesPrivatePattern = $true
        break
      }
    }
    if ($privateLeafNames -contains $leaf -or $matchesPrivatePattern) {
      $trackedPath
    }
  }
  if ($private) {
    throw "Private or generated files are tracked: $($private -join ', ')"
  }

  Write-Host "Repository layout check passed."
} finally {
  Pop-Location
}
```

- [ ] **Step 2: Add negative tests with an isolated Git index**

Create `scripts/repository-layout-check-test.ps1` with this content:

```powershell
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$layoutCheck = Join-Path $PSScriptRoot "repository-layout-check.ps1"
$forbiddenPaths = @(
  "course-export-diagnosis.json",
  "hdu-current-timetable-test.json",
  "hdu-target-timetable.json",
  "logs/run.log",
  "data/course.xlsx",
  "data/legacy.xls",
  "course.json.bak-20260820",
  "HDU-Smart-Course-Agent/config.json",
  "execution-runbook.md",
  "dist/generated.txt",
  "release/package/README.md"
)

Push-Location $repoRoot
try {
  $realIndex = (git rev-parse --git-path index).Trim()
  $blob = (git rev-parse HEAD:README.md).Trim()
  if ($LASTEXITCODE -ne 0 -or -not $realIndex -or -not $blob) {
    throw "Could not resolve Git index test inputs."
  }

  for ($index = 0; $index -lt $forbiddenPaths.Count; $index += 1) {
    $forbiddenPath = $forbiddenPaths[$index]
    $testIndex = Join-Path $env:TEMP "hdu-layout-check-$PID-$index.index"
    $previousIndex = [Environment]::GetEnvironmentVariable("GIT_INDEX_FILE", "Process")
    Copy-Item -LiteralPath $realIndex -Destination $testIndex -Force

    try {
      $env:GIT_INDEX_FILE = $testIndex
      git update-index --add --cacheinfo "100644,$blob,$forbiddenPath"
      if ($LASTEXITCODE -ne 0) {
        throw "Could not add test index entry: $forbiddenPath"
      }

      $previousErrorActionPreference = $ErrorActionPreference
      $ErrorActionPreference = "Continue"
      try {
        powershell -ExecutionPolicy Bypass -File $layoutCheck *> $null
        $layoutExitCode = $LASTEXITCODE
      } finally {
        $ErrorActionPreference = $previousErrorActionPreference
      }
      if ($layoutExitCode -eq 0) {
        throw "Layout check accepted forbidden tracked path: $forbiddenPath"
      }
    } finally {
      if ($null -eq $previousIndex) {
        Remove-Item Env:GIT_INDEX_FILE -ErrorAction SilentlyContinue
      } else {
        $env:GIT_INDEX_FILE = $previousIndex
      }
      if (Test-Path -LiteralPath $testIndex) {
        Remove-Item -LiteralPath $testIndex -Force
      }
    }
  }

  Write-Host "Repository layout negative tests passed."
} finally {
  Pop-Location
}
```

Run before widening the checker:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check-test.ps1
```

Expected before the checker fix: FAIL with `Layout check accepted forbidden tracked path: course-export-diagnosis.json`.

- [ ] **Step 3: Run the positive and negative layout checks**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check.ps1
powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check-test.ps1
```

Expected: `Repository layout check passed.` and `Repository layout negative tests passed.`

- [ ] **Step 4: Add the GitHub Actions workflow**

Create `.github/workflows/ci.yml` with this content:

```yaml
name: CI

on:
  push:
  pull_request:

permissions:
  contents: read

jobs:
  verify:
    runs-on: windows-latest
    steps:
      - name: Check out repository
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.23.x"

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: "22"

      - name: Check repository layout
        shell: powershell
        run: powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check.ps1

      - name: Test repository layout rejections
        shell: powershell
        run: powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check-test.ps1

      - name: Test main module
        shell: powershell
        run: go test -buildvcs=false ./...

      - name: Test Smart Agent module
        shell: powershell
        working-directory: HDU-Smart-Course-Agent
        run: go test -buildvcs=false ./...

      - name: Test scheduler Worker
        shell: powershell
        run: node scripts/scheduler-worker-smoke.js

      - name: Run deterministic teaching-system acceptance
        shell: powershell
        run: powershell -ExecutionPolicy Bypass -File scripts/testlab-acceptance.ps1
```

- [ ] **Step 5: Document the single CI-grade command set**

Add a short `持续集成` subsection under `README.md`'s test section. State that GitHub Actions runs the layout check, both Go module suites, Worker smoke, and deterministic testlab acceptance. Include the local layout command exactly:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check.ps1
```

- [ ] **Step 6: Verify the workflow and commit**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check.ps1
powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check-test.ps1
go test -buildvcs=false ./...
Push-Location HDU-Smart-Course-Agent
try { go test -buildvcs=false ./... } finally { Pop-Location }
node scripts/scheduler-worker-smoke.js
git diff --check
```

Expected: every command passes and the workflow contains no secret, release, or deployment permissions.

Commit:

```powershell
git add .github/workflows/ci.yml scripts/repository-layout-check.ps1 scripts/repository-layout-check-test.ps1 README.md docs/superpowers/plans/2026-08-20-repository-presentation-and-layout-cleanup.md
git commit -m "ci: verify repository layout and both modules"
```

### Task 5: Run Full Source and Release Regression Gates

**Files:**
- Inspect: all files changed in Tasks 1-4.
- Modify: `docs/superpowers/plans/2026-08-20-repository-presentation-and-layout-cleanup.md`
- Generate locally only: ignored files under `dist/` and `release/` through existing scripts.

**Interfaces:**
- Consumes: the reorganized tracked tree and existing release tooling.
- Produces: evidence that source moves did not change browser routes, package contents, Smart Agent behavior, or data boundaries.

- [ ] **Step 1: Run the source-level gate**

Run:

```powershell
git diff --check
powershell -ExecutionPolicy Bypass -File scripts/repository-layout-check.ps1
go test -buildvcs=false ./...
Push-Location HDU-Smart-Course-Agent
try { go test -buildvcs=false ./... } finally { Pop-Location }
node scripts/scheduler-worker-smoke.js
powershell -ExecutionPolicy Bypass -File scripts/testlab-acceptance.ps1
```

Expected: all commands pass; testlab remains loopback-only and uses deterministic fixtures.

- [ ] **Step 2: Run the main browser acceptance from a fresh executable**

Run:

```powershell
$layoutExe = Join-Path $env:TEMP "HDU-Auto-Scheduling-Script-layout-check.exe"
$previousMainExe = [Environment]::GetEnvironmentVariable("HDU_MAIN_EXE", "Process")
$previousMainPort = [Environment]::GetEnvironmentVariable("HDU_MAIN_PORT", "Process")
$portListener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
$portListener.Start()
$mainPort = $portListener.LocalEndpoint.Port
$portListener.Stop()
go build -buildvcs=false -o $layoutExe .
$env:HDU_MAIN_EXE = $layoutExe
$env:HDU_MAIN_PORT = $mainPort.ToString()
try {
  node scripts/main-ui-acceptance.js
} finally {
  if ($null -eq $previousMainExe) {
    Remove-Item Env:HDU_MAIN_EXE -ErrorAction SilentlyContinue
  } else {
    $env:HDU_MAIN_EXE = $previousMainExe
  }
  if ($null -eq $previousMainPort) {
    Remove-Item Env:HDU_MAIN_PORT -ErrorAction SilentlyContinue
  } else {
    $env:HDU_MAIN_PORT = $previousMainPort
  }
  if (Test-Path -LiteralPath $layoutExe) { Remove-Item -LiteralPath $layoutExe -Force }
}
```

Expected: desktop and mobile scenarios pass for no data, course-only data, and course plus personal timetable data; `/`, `/exporter/`, and `/scheduler.html` remain reachable.

- [ ] **Step 3: Rebuild the ignored release package**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1
$version = (Get-Content VERSION -Raw).Trim()
$package = "release/HDU-Auto-Scheduling-Script-v$version"
$manifest = Get-Content "$package/manifest.json" -Raw | ConvertFrom-Json
if ($manifest.version -ne $version) { throw "Manifest version does not match VERSION" }
$missing = $manifest.files | Where-Object { -not (Test-Path -LiteralPath (Join-Path $package $_)) }
if ($missing) { throw "Manifest files are missing: $($missing -join ', ')" }
```

Expected: the package builds from the reorganized source, manifest version matches `VERSION`, and every manifest entry exists. The manifest file list remains unchanged.

- [ ] **Step 4: Run package-level smoke checks on free ports**

Run:

```powershell
$version = (Get-Content VERSION -Raw).Trim()
$package = "release/HDU-Auto-Scheduling-Script-v$version"
powershell -ExecutionPolicy Bypass -File "$package/scripts/release-main-smoke.ps1" -PackageDir $package -Port 6903
$env:HDU_SMART_AGENT_PORT = "6971"
try {
  powershell -ExecutionPolicy Bypass -File "$package/scripts/smart-agent-e2e.ps1"
  node "$package/scripts/smart-agent-ui-smoke.js"
} finally {
  Remove-Item Env:HDU_SMART_AGENT_PORT -ErrorAction SilentlyContinue
}
```

Expected: main smoke, Smart Agent E2E, and Smart Agent UI smoke pass without touching a real `HDU-KillCourse/config.json` or performing a real course operation.

- [ ] **Step 5: Run the final Git boundary audit**

Run:

```powershell
git status --short --branch
git diff --check
git ls-tree --name-only HEAD
git ls-files | rg "(^|/)(course|course-export-diagnosis|personal-schedule|personal-schedule-live|target-schedule|login-config|live-schedule-sync|action-plan|agent-settings|execution-approval|execution-package|execution-log|fallback-recommendations)\.json$|(^|/)hdu-(current|target)-timetable[^/]*\.json$|(^|/)run-killcourse\.bat$|\.(exe|zip|db|db-wal|db-shm)$"
```

Expected: the working tree is clean after the task commits; the root tree contains `web/` instead of eight frontend files and no root Codex brief; the final search reports no tracked personal data, executable, ZIP, or database.

## Explicit Follow-up Plans

The following work is intentionally excluded because each item changes internal architecture rather than repository presentation and should receive its own spec and plan:

- split `HDU-Smart-Course-Agent/main.go` by plan generation, execution authorization, configuration, log parsing, and HTTP transport;
- split `HDU-Smart-Course-Agent/web/app.js` and the root scheduler JavaScript by domain responsibility;
- establish a shared or contract-tested course schema across the two Go modules;
- decide whether the standalone `cmd/course-exporter` Go command can be retired while retaining its `/exporter/` web UI;
- select an open-source license and publish a maintainer-approved vulnerability reporting contact.
