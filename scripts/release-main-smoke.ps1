param(
  [string]$PackageDir = "",
  [int]$Port = 6789,
  [switch]$KeepTemp
)

$ErrorActionPreference = "Stop"

if ($Port -lt 1 -or $Port -gt 65535) {
  throw "Port must be between 1 and 65535."
}

$ScriptDir = $PSScriptRoot
$Root = Split-Path -Parent $ScriptDir
$VersionPath = Join-Path $Root "VERSION"

if ([string]::IsNullOrWhiteSpace($PackageDir)) {
  if (Test-Path -LiteralPath (Join-Path $Root "HDU-Auto-Scheduling-Script.exe")) {
    $PackageDir = $Root
  } else {
    $version = "0.0.0"
    if (Test-Path -LiteralPath $VersionPath) {
      $version = (Get-Content -Path $VersionPath -Raw).Trim()
    }
    $PackageDir = Join-Path $Root "release\HDU-Auto-Scheduling-Script-v$version"
  }
}

$PackageDir = [System.IO.Path]::GetFullPath($PackageDir)
$ExePath = Join-Path $PackageDir "HDU-Auto-Scheduling-Script.exe"
$SampleCoursePath = Join-Path $PackageDir "samples\course.sample.json"
$TempRoot = Join-Path $Root "tmp-release-main-smoke"
$ApiBase = "http://127.0.0.1:$Port"

function Write-Step {
  param([string]$Message)
  Write-Host "[release-main-smoke] $Message"
}

function Assert-File {
  param(
    [string]$Path,
    [string]$Label
  )
  if (-not (Test-Path -LiteralPath $Path)) {
    throw "$Label not found: $Path"
  }
}

function Test-PortFree {
  try {
    Invoke-RestMethod -Uri "$ApiBase/api/status" -TimeoutSec 1 | Out-Null
    return $false
  } catch {
    return $true
  }
}

function Wait-AppStatus {
  $deadline = (Get-Date).AddSeconds(15)
  while ((Get-Date) -lt $deadline) {
    try {
      return Invoke-RestMethod -Uri "$ApiBase/api/status" -TimeoutSec 1
    } catch {
      Start-Sleep -Milliseconds 250
    }
  }
  throw "Main exe did not become ready at $ApiBase."
}

function Stop-App {
  param([System.Diagnostics.Process]$Process)
  if ($null -ne $Process -and -not $Process.HasExited) {
    Stop-Process -Id $Process.Id -Force
    $Process.WaitForExit(5000) | Out-Null
  }
}

function New-SmokeDir {
  param([string]$Name)
  $path = Join-Path $TempRoot $Name
  if (Test-Path -LiteralPath $path) {
    Remove-Item -LiteralPath $path -Recurse -Force
  }
  New-Item -ItemType Directory -Path $path | Out-Null
  Copy-Item -LiteralPath $ExePath -Destination (Join-Path $path "HDU-Auto-Scheduling-Script.exe")
  return $path
}

function Invoke-MainSmokeCase {
  param(
    [string]$Name,
    [switch]$WithCourse
  )

  $caseDir = New-SmokeDir -Name $Name
  if ($WithCourse) {
    Copy-Item -LiteralPath $SampleCoursePath -Destination (Join-Path $caseDir "course.json")
  }

  $oldNoBrowser = $env:HDU_NO_BROWSER
  $oldOutputDir = $env:HDU_OUTPUT_DIR
  $oldMainPort = $env:HDU_MAIN_PORT
  $env:HDU_NO_BROWSER = "1"
  $env:HDU_OUTPUT_DIR = $caseDir
  $env:HDU_MAIN_PORT = [string]$Port
  $stdoutPath = Join-Path $caseDir "main.stdout.log"
  $stderrPath = Join-Path $caseDir "main.stderr.log"
  $process = Start-Process -FilePath (Join-Path $caseDir "HDU-Auto-Scheduling-Script.exe") -WorkingDirectory $caseDir -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath -PassThru
  try {
    try {
      $status = Wait-AppStatus
    } catch {
      $stdout = if (Test-Path -LiteralPath $stdoutPath) { Get-Content -LiteralPath $stdoutPath -Raw } else { "" }
      $stderr = if (Test-Path -LiteralPath $stderrPath) { Get-Content -LiteralPath $stderrPath -Raw } else { "" }
      throw "$($_.Exception.Message) stdout=$stdout stderr=$stderr"
    }
    $html = Invoke-WebRequest -Uri "$ApiBase/" -UseBasicParsing -TimeoutSec 2
    if ($html.StatusCode -ne 200 -or $html.Content.Length -lt 500) {
      throw "Main page did not return expected HTML for case $Name."
    }

    if ($WithCourse) {
      if ($status.ready -ne $true -or $status.count -lt 1) {
        throw "Expected ready=true with sample course.json for case $Name."
      }
      if ($null -eq $status.PSObject.Properties["personalExported"]) {
        throw "Status response is missing personalExported=false for case $Name."
      }
    } else {
      if ($status.ready -eq $true) {
        throw "Expected ready=false without course.json for case $Name."
      }
    }

    return [ordered]@{
      name = $Name
      ready = $status.ready
      count = $status.count
      personalExported = $status.personalExported
      htmlBytes = $html.Content.Length
    }
  } finally {
    Stop-App -Process $process
    if ($null -eq $oldNoBrowser) {
      Remove-Item Env:\HDU_NO_BROWSER -ErrorAction SilentlyContinue
    } else {
      $env:HDU_NO_BROWSER = $oldNoBrowser
    }
    if ($null -eq $oldOutputDir) {
      Remove-Item Env:\HDU_OUTPUT_DIR -ErrorAction SilentlyContinue
    } else {
      $env:HDU_OUTPUT_DIR = $oldOutputDir
    }
    if ($null -eq $oldMainPort) {
      Remove-Item Env:\HDU_MAIN_PORT -ErrorAction SilentlyContinue
    } else {
      $env:HDU_MAIN_PORT = $oldMainPort
    }
  }
}

Assert-File -Path $ExePath -Label "Release main exe"
Assert-File -Path $SampleCoursePath -Label "Sample course data"

if (-not (Test-PortFree)) {
  throw "Port $Port is already in use. Close the running main assistant or choose another port, then retry."
}

if (Test-Path -LiteralPath $TempRoot) {
  Remove-Item -LiteralPath $TempRoot -Recurse -Force
}
New-Item -ItemType Directory -Path $TempRoot | Out-Null

try {
  Write-Step "Checking launch without course.json..."
  $missingCourse = Invoke-MainSmokeCase -Name "missing-course"
  Write-Step "Checking launch with course.json but without personal-schedule.json..."
  $courseOnly = Invoke-MainSmokeCase -Name "course-only" -WithCourse
  Write-Step "Passed."
  [ordered]@{
    packageDir = $PackageDir
    missingCourse = $missingCourse
    courseOnly = $courseOnly
  } | ConvertTo-Json -Depth 6
} finally {
  if (-not $KeepTemp -and (Test-Path -LiteralPath $TempRoot)) {
    Remove-Item -LiteralPath $TempRoot -Recurse -Force
  }
}
