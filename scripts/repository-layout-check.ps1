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
