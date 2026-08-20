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

function Start-TestLabForScenario {
  param([string]$Name)
  Stop-TestLab
  $port = Get-Random -Minimum 22000 -Maximum 42000
  $baseUrl = "http://127.0.0.1:$port"
  $scenarioDir = Join-Path $TempRoot $Name
  New-Item -ItemType Directory -Force -Path $scenarioDir | Out-Null
  $script:ServerOut = Join-Path $scenarioDir "server.out.txt"
  $script:ServerErr = Join-Path $scenarioDir "server.err.txt"
  $fixtureDir = Join-Path $Root "testdata"
  $serverArguments = "-mode serve -listen `"127.0.0.1:$port`" -scenario `"$Name`" -fixtures `"$fixtureDir`""
  $script:Server = Start-Process -FilePath $LabExe -ArgumentList $serverArguments -PassThru -NoNewWindow -RedirectStandardOutput $script:ServerOut -RedirectStandardError $script:ServerErr
  Wait-TestLab $baseUrl
  return $baseUrl
}

function Invoke-KillCourseMockScenario {
  param([string]$Name, [string]$ExpectSelectFlag)
  $baseUrl = Start-TestLabForScenario -Name $Name
  $selectIndex = Invoke-WebRequest -Method Get -Uri "$baseUrl/xsxk/zzxkyzb_cxZzxkYzbIndex.html" -UseBasicParsing -TimeoutSec 5
  Assert-True ($selectIndex.Content -match "xkkz01") "Scenario $Name select-index page is missing XkkzId."
  $doJxbRawText = (Invoke-WebRequest -Method Post -Uri "$baseUrl/xsxk/zzxkyzbjk_cxJxbWithKchZzxkYzb.html" -UseBasicParsing -TimeoutSec 5).Content
  [System.Reflection.Assembly]::LoadWithPartialName("System.Web.Extensions") | Out-Null
  $serializer = New-Object System.Web.Script.Serialization.JavaScriptSerializer
  $doJxb = @($serializer.DeserializeObject($doJxbRawText))
  Assert-True ($doJxb.Count -ge 2) "Scenario $Name do_jxb_id list is too small ($doJxbRawText)."
  Assert-True (@($doJxb | Where-Object { $_.do_jxb_id -eq "do-kill-jxb-01" }).Count -eq 1) "Scenario $Name do_jxb_id mapping is missing."
  $select = Invoke-RestMethod -Method Post -Uri "$baseUrl/xsxk/zzxkyzbjk_xkBcZyZzxkYzb.html" -TimeoutSec 5
  Assert-True ($select.flag -eq $ExpectSelectFlag) "Scenario $Name select flag=$($select.flag), want $ExpectSelectFlag."
  $drop = Invoke-RestMethod -Method Post -Uri "$baseUrl/xsxk/zzxkyzb_tuikBcZzxkYzb.html" -TimeoutSec 5
  Assert-True ("$drop" -eq "1") "Scenario $Name drop result is not \"1\": $drop"
  Stop-TestLab
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
  Invoke-Scenario -Name "course-shape-drift" -ExpectedSuccess $false -ExpectedDiagnosis $true | Out-Null
  Invoke-Scenario -Name "timeout" -ExpectedSuccess $false | Out-Null
  Invoke-Scenario -Name "personal-failure" -ExpectedSuccess $true -ExpectedPersonal $false | Out-Null
  Invoke-KillCourseMockScenario -Name "killcourse" -ExpectSelectFlag "1"
  Invoke-KillCourseMockScenario -Name "killcourse-fail" -ExpectSelectFlag "0"

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

  Write-Step "Checking Smart Agent class schedule aggregation..."
  node scripts\smart-agent-class-schedule-check.js
  if ($LASTEXITCODE -ne 0) { throw "Smart Agent class schedule check failed." }

  Write-Step "Checking Smart Agent live-capacity capture against mock..."
  $captureBase = Start-TestLabForScenario -Name "killcourse"
  $env:HDU_TESTLAB_BASE = $captureBase
  $env:HDU_SMART_AGENT_EXE = $smartAgentExe
  try {
    node scripts\smart-agent-live-capacity-capture-check.js
    if ($LASTEXITCODE -ne 0) { throw "Smart Agent live-capacity capture check failed." }
  } finally {
    Stop-TestLab
    Remove-Item Env:HDU_TESTLAB_BASE -ErrorAction SilentlyContinue
  }

  Write-Step "Checking Smart Agent refresh contract..."
  node scripts\smart-agent-refresh-contract-test.js
  if ($LASTEXITCODE -ne 0) { throw "Smart Agent refresh contract failed." }

  Write-Step "Checking Smart Agent refresh backoff helper..."
  node scripts\smart-agent-backoff-check.js
  if ($LASTEXITCODE -ne 0) { throw "Smart Agent refresh backoff check failed." }

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
