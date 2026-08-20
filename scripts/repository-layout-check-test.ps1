$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$layoutCheck = Join-Path $PSScriptRoot "repository-layout-check.ps1"
$killCourseDirName = -join (0x9009, 0x8BFE, 0x811A, 0x672C | ForEach-Object { [char]$_ })
$forbiddenPaths = @(
  "course-export-diagnosis.json",
  "hdu-current-timetable-test.json",
  "hdu-target-timetable.json",
  "logs/run.log",
  "data/course.xlsx",
  "data/legacy.xls",
  "course.json.bak-20260820",
  "HDU-Smart-Course-Agent/config.json",
  "HDU-Smart-Course-Agent/third_party/HDU-KillCourse/config.json",
  "$killCourseDirName/config.json",
  "execution-runbook.md",
  "dist/generated.txt",
  "release/package/README.md"
)

Push-Location $repoRoot
try {
  $nonAscii = @([System.IO.File]::ReadAllText($layoutCheck).ToCharArray() | Where-Object { [int]$_ -gt 127 })
  if ($nonAscii.Count -gt 0) {
    throw "Layout check must contain only ASCII so Windows PowerShell 5.1 parses it consistently."
  }

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
