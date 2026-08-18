param(
  [switch]$KeepTemp
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$TempRoot = Join-Path $Root "tmp-testlab-acceptance"
$LabExe = Join-Path $TempRoot "hdu-testlab.exe"
$script:Server = $null

function Write-Step {
  param([string]$Message)
  Write-Host "[testlab] $Message"
}

function Assert-True {
  param([bool]$Condition, [string]$Message)
  if (-not $Condition) { throw $Message }
}

function Get-FreeLoopbackPort {
  $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
  try {
    $listener.Start()
    return $listener.LocalEndpoint.Port
  } finally {
    $listener.Stop()
  }
}

function Stop-TestLab {
  if ($null -ne $script:Server) {
    if (-not $script:Server.HasExited) {
      Stop-Process -Id $script:Server.Id -Force
      $script:Server.WaitForExit(5000) | Out-Null
    }
    $script:Server.Dispose()
    $script:Server = $null
  }
}

function Wait-TestLab {
  param([string]$BaseUrl)
  $deadline = (Get-Date).AddSeconds(12)
  while ((Get-Date) -lt $deadline) {
    try {
      $health = Invoke-RestMethod -Method Get -Uri "$BaseUrl/health" -TimeoutSec 1
      if ($health.ok -eq $true) { return }
    } catch {}
    Start-Sleep -Milliseconds 150
  }
  throw "Mock teaching system did not become ready at $BaseUrl."
}

function Invoke-Scenario {
  param(
    [string]$Name,
    [bool]$ExpectedSuccess,
    [bool]$ExpectedDiagnosis = $false,
    [bool]$ExpectedPersonal = $false
  )

  Stop-TestLab
  $port = Get-Random -Minimum 22000 -Maximum 42000
  $baseUrl = "http://127.0.0.1:$port"
  $scenarioDir = Join-Path $TempRoot $Name
  $stdout = Join-Path $scenarioDir "export.json"
  $stderr = Join-Path $scenarioDir "export.stderr.txt"
  $serverOut = Join-Path $scenarioDir "server.out.txt"
  $serverErr = Join-Path $scenarioDir "server.err.txt"
  New-Item -ItemType Directory -Force -Path $scenarioDir | Out-Null

  $fixtureDir = Join-Path $Root "testdata"
  $serverArguments = "-mode serve -listen `"127.0.0.1:$port`" -scenario `"$Name`" -fixtures `"$fixtureDir`""
  $script:Server = Start-Process -FilePath $LabExe -ArgumentList $serverArguments -PassThru -NoNewWindow -RedirectStandardOutput $serverOut -RedirectStandardError $serverErr
  Wait-TestLab $baseUrl

  # Expected failure scenarios return a non-zero native exit code. Temporarily
  # keep PowerShell from treating that expected condition as a terminating error.
  $previousErrorActionPreference = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  $previousOutputDirectory = [Environment]::GetEnvironmentVariable("HDU_OUTPUT_DIR", "Process")
  $env:HDU_OUTPUT_DIR = $scenarioDir
  try {
    & $LabExe -mode export -base $baseUrl -output $scenarioDir -timeout 300ms 1> $stdout 2> $stderr
    $exitCode = $LASTEXITCODE
  } finally {
    if ($null -eq $previousOutputDirectory) {
      Remove-Item Env:HDU_OUTPUT_DIR -ErrorAction SilentlyContinue
    } else {
      $env:HDU_OUTPUT_DIR = $previousOutputDirectory
    }
    $ErrorActionPreference = $previousErrorActionPreference
  }
  $response = Get-Content -LiteralPath $stdout -Raw -Encoding UTF8 | ConvertFrom-Json
  Assert-True (($exitCode -eq 0) -eq $ExpectedSuccess) "Scenario $Name exit=$exitCode, expected success=$ExpectedSuccess. $(Get-Content -LiteralPath $stderr -Raw)"
  Assert-True (($response.ok -eq $true) -eq $ExpectedSuccess) "Scenario $Name JSON result does not match expected outcome."
  Assert-True ((Test-Path -LiteralPath (Join-Path $scenarioDir "course-export-diagnosis.json")) -eq $ExpectedDiagnosis) "Scenario $Name diagnosis-file result is wrong."

  if ($ExpectedSuccess) {
    Assert-True (Test-Path -LiteralPath (Join-Path $scenarioDir "course.json")) "Scenario $Name did not write course.json."
    $course = Get-Content -LiteralPath (Join-Path $scenarioDir "course.json") -Raw -Encoding UTF8 | ConvertFrom-Json
    Assert-True ($course.items.Count -ge 8) "Scenario $Name exported too few courses."
    Assert-True (($response.status.personalExported -eq $true) -eq $ExpectedPersonal) "Scenario $Name personal export status is wrong."
    Assert-True ((Test-Path -LiteralPath (Join-Path $scenarioDir "personal-schedule.json")) -eq $ExpectedPersonal) "Scenario $Name personal timetable file result is wrong."
  } else {
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $scenarioDir "course.json"))) "Scenario $Name unexpectedly wrote course.json."
  }

  Stop-TestLab
  return $scenarioDir
}

try {
  Set-Location $Root
  if (Test-Path -LiteralPath $TempRoot) { Remove-Item -LiteralPath $TempRoot -Recurse -Force }
  New-Item -ItemType Directory -Path $TempRoot | Out-Null

  Write-Step "Building the loopback-only mock teaching system..."
  $env:GOCACHE = Join-Path $Root ".gocache"
  go build -buildvcs=false -o $LabExe .\cmd\hdu-testlab
  if ($LASTEXITCODE -ne 0) { throw "Could not build hdu-testlab." }

  $successDir = Invoke-Scenario -Name "success" -ExpectedSuccess $true -ExpectedPersonal $true
  Invoke-Scenario -Name "bad-password" -ExpectedSuccess $false | Out-Null
  Invoke-Scenario -Name "forbidden" -ExpectedSuccess $false | Out-Null
  Invoke-Scenario -Name "malformed-course" -ExpectedSuccess $false -ExpectedDiagnosis $true | Out-Null
  Invoke-Scenario -Name "empty-course" -ExpectedSuccess $false -ExpectedDiagnosis $true | Out-Null
  Invoke-Scenario -Name "timeout" -ExpectedSuccess $false | Out-Null
  Invoke-Scenario -Name "personal-failure" -ExpectedSuccess $true -ExpectedPersonal $false | Out-Null

  Write-Step "Using mock-exported course data in the scheduler worker smoke test..."
  $env:HDU_COURSE_FIXTURE = Join-Path $successDir "course.json"
  node scripts\scheduler-worker-smoke.js
  if ($LASTEXITCODE -ne 0) { throw "Scheduler worker smoke test failed against exported mock data." }

  Write-Step "Building and checking the main application desktop/mobile UI..."
  $mainExe = Join-Path $TempRoot "HDU-Auto-Scheduling-Script.exe"
  go build -buildvcs=false -o $mainExe .
  if ($LASTEXITCODE -ne 0) { throw "Could not build the main application for UI acceptance." }
  $smartAgentExe = Join-Path $TempRoot "HDU-Smart-Course-Agent.exe"
  Push-Location (Join-Path $Root "HDU-Smart-Course-Agent")
  try {
    go build -buildvcs=false -o $smartAgentExe .
    if ($LASTEXITCODE -ne 0) { throw "Could not build Smart Agent for acceptance." }
  } finally {
    Pop-Location
  }
  $previousMainPort = [Environment]::GetEnvironmentVariable("HDU_MAIN_PORT", "Process")
  $previousSmartAgentPort = [Environment]::GetEnvironmentVariable("HDU_SMART_AGENT_PORT", "Process")
  $previousSmartAgentExe = [Environment]::GetEnvironmentVariable("HDU_SMART_AGENT_EXE", "Process")
  $env:HDU_MAIN_PORT = [string](Get-FreeLoopbackPort)
  $env:HDU_SMART_AGENT_PORT = [string](Get-FreeLoopbackPort)
  $env:HDU_SMART_AGENT_EXE = $smartAgentExe
  $env:HDU_MAIN_EXE = $mainExe
  node scripts\main-ui-acceptance.js
  if ($LASTEXITCODE -ne 0) { throw "Main application UI acceptance failed." }

  Write-Step "Using mock-exported course data in Smart Agent end-to-end validation..."
  powershell -ExecutionPolicy Bypass -File scripts\smart-agent-e2e.ps1
  if ($LASTEXITCODE -ne 0) { throw "Smart Agent end-to-end test failed against exported mock data." }

  Write-Step "Checking Smart Agent UI resources and APIs..."
  node scripts\smart-agent-ui-smoke.js
  if ($LASTEXITCODE -ne 0) { throw "Smart Agent UI/API smoke test failed." }

  Write-Step "Checking Smart Agent refresh contract..."
  node scripts\smart-agent-refresh-contract-test.js
  if ($LASTEXITCODE -ne 0) { throw "Smart Agent refresh contract failed." }

  Remove-Item Env:HDU_COURSE_FIXTURE -ErrorAction SilentlyContinue
  Remove-Item Env:HDU_MAIN_EXE -ErrorAction SilentlyContinue
  Remove-Item Env:HDU_SMART_AGENT_EXE -ErrorAction SilentlyContinue

  Write-Step "All local teaching-system acceptance scenarios passed."
} finally {
  Stop-TestLab
  Remove-Item Env:HDU_COURSE_FIXTURE -ErrorAction SilentlyContinue
  Remove-Item Env:HDU_MAIN_EXE -ErrorAction SilentlyContinue
  if ($null -eq $previousSmartAgentExe) {
    Remove-Item Env:HDU_SMART_AGENT_EXE -ErrorAction SilentlyContinue
  } else {
    $env:HDU_SMART_AGENT_EXE = $previousSmartAgentExe
  }
  if ($null -eq $previousMainPort) {
    Remove-Item Env:HDU_MAIN_PORT -ErrorAction SilentlyContinue
  } else {
    $env:HDU_MAIN_PORT = $previousMainPort
  }
  if ($null -eq $previousSmartAgentPort) {
    Remove-Item Env:HDU_SMART_AGENT_PORT -ErrorAction SilentlyContinue
  } else {
    $env:HDU_SMART_AGENT_PORT = $previousSmartAgentPort
  }
  if (-not $KeepTemp -and (Test-Path -LiteralPath $TempRoot)) {
    Remove-Item -LiteralPath $TempRoot -Recurse -Force
  }
}
