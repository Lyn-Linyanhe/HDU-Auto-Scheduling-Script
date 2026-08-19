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
  "run-killcourse.bat"
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

  $private = $tracked | Where-Object {
    $leaf = Split-Path -Leaf $_
    $privateLeafNames -contains $leaf -or $_ -match '\.(exe|zip|db|db-wal|db-shm)$'
  }
  if ($private) {
    throw "Private or generated files are tracked: $($private -join ', ')"
  }

  Write-Host "Repository layout check passed."
} finally {
  Pop-Location
}
