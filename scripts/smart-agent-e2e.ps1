param(
  [switch]$KeepTemp
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$SourceAgentDir = Join-Path $Root "HDU-Smart-Course-Agent"
$DistAgentDir = Join-Path $Root "dist"
$ReleaseAgentDir = Join-Path $Root "smart-agent"
if (-not [string]::IsNullOrWhiteSpace([string]$env:HDU_SMART_AGENT_EXE)) {
  $AgentExe = [string]$env:HDU_SMART_AGENT_EXE
} elseif (Test-Path -LiteralPath (Join-Path $SourceAgentDir "HDU-Smart-Course-Agent.exe")) {
  $AgentDir = $SourceAgentDir
  $AgentExe = Join-Path $AgentDir "HDU-Smart-Course-Agent.exe"
} elseif (Test-Path -LiteralPath (Join-Path $DistAgentDir "HDU-Smart-Course-Agent.exe")) {
  $AgentDir = $DistAgentDir
  $AgentExe = Join-Path $AgentDir "HDU-Smart-Course-Agent.exe"
} elseif (Test-Path -LiteralPath (Join-Path $ReleaseAgentDir "HDU-Smart-Course-Agent.exe")) {
  $AgentDir = $ReleaseAgentDir
  $AgentExe = Join-Path $AgentDir "HDU-Smart-Course-Agent.exe"
} else {
  $AgentDir = $SourceAgentDir
  $AgentExe = Join-Path $AgentDir "HDU-Smart-Course-Agent.exe"
}
$TempRoot = Join-Path $Root "tmp-smart-agent-e2e"
$TempSchedulerDir = Join-Path $TempRoot "Scheduler"
$FakeKillDir = Join-Path $TempRoot "KillCourse"
$FakeEntryDir = Join-Path $FakeKillDir "cmd\HDU-KillCourse"
$SmartAgentPort = [string]$env:HDU_SMART_AGENT_PORT
if ([string]::IsNullOrWhiteSpace($SmartAgentPort) -or $SmartAgentPort -notmatch '^[1-9]\d{0,4}$' -or [int]$SmartAgentPort -gt 65535) {
  $SmartAgentPort = "6899"
}
$ApiBase = "http://127.0.0.1:$SmartAgentPort"
$AgentProcess = $null

function Write-Step {
  param([string]$Message)
  Write-Host "[smart-agent-e2e] $Message"
}

function New-Utf8NoBomEncoding {
  return New-Object System.Text.UTF8Encoding($false)
}

function Write-JsonFile {
  param(
    [string]$Path,
    [object]$Value
  )
  $json = $Value | ConvertTo-Json -Depth 30
  [System.IO.File]::WriteAllText($Path, $json, (New-Utf8NoBomEncoding))
}

function New-Text {
  param([int[]]$CodePoints)
  return -join ($CodePoints | ForEach-Object { [char]$_ })
}

function Invoke-AgentJson {
  param(
    [string]$Method,
    [string]$Path,
    [object]$Body = $null
  )

  $uri = "$ApiBase$Path"
  if ($null -eq $Body) {
    return Invoke-RestMethod -Method $Method -Uri $uri
  }

  $json = $Body | ConvertTo-Json -Depth 60
  return Invoke-RestMethod -Method $Method -Uri $uri -ContentType "application/json; charset=utf-8" -Body $json
}

function Assert-Ok {
  param(
    [object]$Response,
    [string]$Step
  )
  if ($null -eq $Response -or $Response.ok -ne $true) {
    $errorText = ""
    if ($null -ne $Response -and $Response.PSObject.Properties.Name -contains "error") {
      $errorText = $Response.error
    }
    if ($null -ne $Response -and $null -ne $Response.dryRun -and $null -ne $Response.dryRun.events) {
      $events = $Response.dryRun.events | ForEach-Object { "[$($_.level)] $($_.message)" }
      if ($events.Count -gt 0) {
        $errorText = "$errorText`n$($events -join "`n")"
      }
    }
    throw "$Step failed. $errorText"
  }
}

function Wait-Agent {
  $deadline = (Get-Date).AddSeconds(20)
  while ((Get-Date) -lt $deadline) {
    try {
      return Invoke-AgentJson -Method "Get" -Path "/api/status"
    } catch {
      Start-Sleep -Milliseconds 300
    }
  }
  throw "Smart Agent did not become ready at $ApiBase."
}

function Test-AgentPortFree {
  try {
    Invoke-RestMethod -Uri "$ApiBase/api/status" -TimeoutSec 1 | Out-Null
    return $false
  } catch {
    return $true
  }
}

function Stop-StartedAgent {
  if ($null -ne $AgentProcess -and -not $AgentProcess.HasExited) {
    Stop-Process -Id $AgentProcess.Id -Force
    $AgentProcess.WaitForExit(5000) | Out-Null
  }
}

try {
  Set-Location $Root

  $SampleCoursePath = Join-Path $Root "samples\course.sample.json"
  if (-not (Test-Path -LiteralPath $SampleCoursePath)) {
    $SampleCoursePath = Join-Path $Root "testdata\course.sample.json"
  }
  $SourceCoursePath = if ([string]::IsNullOrWhiteSpace($env:HDU_COURSE_FIXTURE)) {
    $SampleCoursePath
  } else {
    $env:HDU_COURSE_FIXTURE
  }
  if (-not (Test-Path -LiteralPath $SourceCoursePath) -and -not (Test-Path -LiteralPath $SampleCoursePath)) {
    throw "course.json or samples/course.sample.json is required for this e2e test."
  }

  if (-not (Test-AgentPortFree)) {
    throw "Port 6899 is already in use. Close the running Smart Agent first, then retry."
  }

  if (-not (Test-Path -LiteralPath $AgentExe)) {
    Write-Step "Smart Agent exe not found. Building it first..."
    Push-Location $AgentDir
    try {
      go build -o "HDU-Smart-Course-Agent.exe" .
    } finally {
      Pop-Location
    }
  }

  if (Test-Path -LiteralPath $TempRoot) {
    Remove-Item -LiteralPath $TempRoot -Recurse -Force
  }
  New-Item -ItemType Directory -Path $TempSchedulerDir | Out-Null
  New-Item -ItemType Directory -Path $FakeEntryDir | Out-Null
  New-Item -ItemType Directory -Path (Join-Path $FakeKillDir "log_files") | Out-Null

  if (Test-Path -LiteralPath $SourceCoursePath) {
    Copy-Item -LiteralPath $SourceCoursePath -Destination (Join-Path $TempSchedulerDir "course.json")
  } else {
    Copy-Item -LiteralPath $SampleCoursePath -Destination (Join-Path $TempSchedulerDir "course.json")
  }

  [System.IO.File]::WriteAllText(
    (Join-Path $FakeEntryDir "main.go"),
    "package main`nfunc main() {}`n",
    (New-Utf8NoBomEncoding)
  )

  $fakeConfig = [ordered]@{
    cas_login = [ordered]@{
      username = "24000000"
      password = "secret"
    }
    cookies = [ordered]@{
      enabled = "0"
    }
    course = [ordered]@{}
    wait_course = [ordered]@{
      enabled = "1"
      interval = 30
    }
    time = [ordered]@{
      XueNian = "2026"
      XueQi = "1"
    }
    start_time = "2026-07-20 12:00:00"
  }
  Write-JsonFile -Path (Join-Path $FakeKillDir "config.json") -Value $fakeConfig

  Write-Step "Starting Smart Agent..."
  $env:HDU_AGENT_NO_BROWSER = "1"
  $AgentProcess = Start-Process -FilePath $AgentExe -WorkingDirectory $TempRoot -WindowStyle Hidden -PassThru
  Wait-Agent | Out-Null

  Write-Step "Pointing Smart Agent to the temporary KillCourse directory..."
  $settingsResp = Invoke-AgentJson -Method "Post" -Path "/api/settings" -Body @{
    schedulerDir = $TempSchedulerDir
    killCourseDir = $FakeKillDir
  }
  Assert-Ok -Response $settingsResp -Step "settings"

  $coursePayload = Invoke-AgentJson -Method "Get" -Path "/api/course"
  if ($null -eq $coursePayload.items -or $coursePayload.items.Count -lt 4) {
    throw "course.json does not contain enough course items for e2e."
  }

  $optionsResp = Invoke-AgentJson -Method "Get" -Path "/api/course-options"
  Assert-Ok -Response $optionsResp -Step "course-options"
  if ($null -eq $optionsResp.items -or $optionsResp.items.Count -lt 4 -or $optionsResp.schemaVersion -ne 1) {
    throw "course-options did not return the expected schema and item count."
  }
  $firstDisplayCode = [string]$optionsResp.items[0].displayCode
  if ([string]::IsNullOrWhiteSpace($firstDisplayCode)) {
    throw "course-options returned an item without displayCode."
  }

  $schedulePath = "/api/class-schedule?displayCode=$([uri]::EscapeDataString($firstDisplayCode))"
  $scheduleResp = Invoke-AgentJson -Method "Get" -Path $schedulePath
  Assert-Ok -Response $scheduleResp -Step "class-schedule"
  if ($null -eq $scheduleResp.items -or $scheduleResp.items.Count -lt 1 -or $scheduleResp.items[0].displayCode -ne $firstDisplayCode) {
    throw "class-schedule did not return the requested teaching class."
  }

  $capacityResp = Invoke-AgentJson -Method "Get" -Path "/api/course-capacity"
  Assert-Ok -Response $capacityResp -Step "course-capacity"
  if ($null -eq $capacityResp.items -or $capacityResp.items.Count -lt 4) {
    throw "course-capacity did not return the expected item count."
  }
  if ($capacityResp.stale -ne $true -or [string]::IsNullOrWhiteSpace([string]$capacityResp.sourceUpdatedAt)) {
    throw "course-capacity must identify the local snapshot as stale and expose sourceUpdatedAt."
  }
  $firstCapacity = $capacityResp.items[0]
  if ($null -ne $firstCapacity.capacity -and [double]$firstCapacity.capacity -lt 0) {
    throw "course-capacity returned a negative capacity count."
  }
  if ($null -ne $firstCapacity.remaining -and [double]$firstCapacity.remaining -lt 0) {
    throw "course-capacity returned a negative remaining count."
  }

  # Use two adjacent real courses whose current fixture does not force a time conflict.
  # The old fourth-item choice overlaps the second item in the current 2026-2027 dataset.
  $targetItems = @($coursePayload.items[1], $coursePayload.items[2])
  $liveResp = Invoke-AgentJson -Method "Post" -Path "/api/live-schedule" -Body @{
    payload = @{
      schemaVersion = 1
      source = "smart-agent-e2e-live"
      term = $coursePayload.term
      items = @($coursePayload.items[0])
    }
  }
  Assert-Ok -Response $liveResp -Step "live-schedule"
  if ($liveResp.sync.liveCount -ne 1 -or -not (Test-Path -LiteralPath (Join-Path $TempSchedulerDir "personal-schedule-live.json"))) {
    throw "live-schedule did not persist the expected snapshot."
  }

  $planResp = Invoke-AgentJson -Method "Post" -Path "/api/plan" -Body @{
    targetPayload = @{
      schemaVersion = 1
      source = "smart-agent-e2e"
      term = $coursePayload.term
      items = $targetItems
    }
    lockedCodes = @()
    writeActionPlan = $false
    writeKillCourseConfig = $true
  }
  Assert-Ok -Response $planResp -Step "plan"
  if ($planResp.configBlocked -eq $true) {
    $blockers = $planResp.blockers | ForEach-Object { "[$($_.level)] $($_.message)" }
    Write-Host "[smart-agent-e2e] Plan blockers: $($blockers -join '; ')"
  }

  $dryResp = Invoke-AgentJson -Method "Post" -Path "/api/execution/dry-run" -Body @{
    plan = $planResp.plan
    generatedConfig = $planResp.generatedConfig
  }
  Assert-Ok -Response $dryResp -Step "dry-run"
  if ($dryResp.dryRun.canExecute -ne $true) {
    $events = $dryResp.dryRun.events | ForEach-Object { "[$($_.level)] $($_.message)" }
    throw "dry-run did not pass execution readiness.`n$($events -join "`n")"
  }

  $authResp = Invoke-AgentJson -Method "Post" -Path "/api/execution/authorize" -Body @{
    plan = $planResp.plan
    generatedConfig = $planResp.generatedConfig
    confirmationPhrase = $dryResp.dryRun.confirmationPhrase
  }
  Assert-Ok -Response $authResp -Step "authorize"

  $packageResp = Invoke-AgentJson -Method "Post" -Path "/api/execution/package" -Body @{
    plan = $planResp.plan
    generatedConfig = $planResp.generatedConfig
    authorization = $authResp.authorization
  }
  Assert-Ok -Response $packageResp -Step "package"

  $logPath = Join-Path $FakeKillDir "log_files\app.log"
  $processingCourse = New-Text @(27491,22312,22788,29702,35838,31243,58)
  $courseNameLabel = New-Text @(35838,31243,21517,31216,58)
  $selectSuccess = New-Text @(36873,35838,25104,21151)
  $selectFailedFull = (New-Text @(36873,35838,22833,36133,58,32,20154,25968,21487,33021,24050,28385))
  $logLines = @(
    "2026/07/08 12:00:00.000000 [INFO] $processingCourse $($targetItems[0].displayCode)",
    "2026/07/08 12:00:01.000000 [INFO] $courseNameLabel $($targetItems[0].courseName)",
    "2026/07/08 12:00:02.000000 [INFO] $selectSuccess",
    "2026/07/08 12:00:03.000000 [INFO] $processingCourse $($targetItems[1].displayCode)",
    "2026/07/08 12:00:04.000000 [INFO] $courseNameLabel $($targetItems[1].courseName)",
    "2026/07/08 12:00:05.000000 [ERROR] $selectFailedFull"
  )
  [System.IO.File]::WriteAllLines($logPath, $logLines, (New-Utf8NoBomEncoding))

  $logResp = Invoke-AgentJson -Method "Post" -Path "/api/execution/parse-log" -Body @{
    plan = $planResp.plan
    generatedConfig = $planResp.generatedConfig
    writeExecutionLog = $true
  }
  Assert-Ok -Response $logResp -Step "parse-log"
  if ($logResp.log.summary.total -ne 2 -or $logResp.log.summary.success -ne 1 -or $logResp.log.summary.failed -ne 1) {
    $logDebug = $logResp.log | ConvertTo-Json -Depth 20
    throw "Unexpected execution log summary.`n$logDebug"
  }
  if ($logResp.refreshAfterSuccess -ne $true) {
    throw "Successful select execution must request a personal schedule refresh."
  }

  $fallbackResp = Invoke-AgentJson -Method "Post" -Path "/api/execution/fallback-recommendations" -Body @{
    plan = $planResp.plan
    executionLog = $logResp.log
    writeFallbackRecommendations = $true
  }
  Assert-Ok -Response $fallbackResp -Step "fallback-recommendations"
  if ($fallbackResp.recommendations.summary.failedSelectCount -lt 1) {
    throw "Expected at least one failed select recommendation item."
  }

  Write-Step "Resetting temporary settings..."
  Invoke-AgentJson -Method "Delete" -Path "/api/settings" | Out-Null

  $summary = [ordered]@{
    targetCount = $planResp.plan.target.Count
    liveCount = $liveResp.sync.liveCount
    optionsCount = $optionsResp.items.Count
    scheduleCount = $scheduleResp.items.Count
    capacityUnknown = ($null -eq $capacityResp.items[0].capacity -and $null -eq $capacityResp.items[0].remaining)
    liveHasDrift = $liveResp.sync.hasDrift
    selectCount = $planResp.plan.select.Count
    dropCount = $planResp.plan.drop.Count
    dryRunCanExecute = $dryResp.dryRun.canExecute
    packageCommand = $packageResp.package.command
    logTotal = $logResp.log.summary.total
    logSuccess = $logResp.log.summary.success
    logFailed = $logResp.log.summary.failed
    fallbackItems = $fallbackResp.recommendations.summary.failedSelectCount
    fallbackWithOptions = $fallbackResp.recommendations.summary.withOptions
  }

  Write-Step "Passed."
  $summary | ConvertTo-Json -Depth 8
} finally {
  try {
    Invoke-AgentJson -Method "Delete" -Path "/api/settings" | Out-Null
  } catch {
  }
  Stop-StartedAgent
  Remove-Item Env:\HDU_AGENT_NO_BROWSER -ErrorAction SilentlyContinue
  if (-not $KeepTemp -and (Test-Path -LiteralPath $TempRoot)) {
    Remove-Item -LiteralPath $TempRoot -Recurse -Force
  }
}
