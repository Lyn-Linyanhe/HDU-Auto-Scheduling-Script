param(
  [string]$Version = "",
  [switch]$SkipBuild,
  [switch]$NoZip
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

function Invoke-Native {
  param(
    [string]$FilePath,
    [string[]]$Arguments
  )

  & $FilePath @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "Command failed with exit code ${LASTEXITCODE}: $FilePath $($Arguments -join ' ')"
  }
}

$GoCache = Join-Path $Root ".gocache"
New-Item -ItemType Directory -Force -Path $GoCache | Out-Null
$env:GOCACHE = $GoCache

if ([string]::IsNullOrWhiteSpace($Version)) {
  $Version = (Get-Content -Path "VERSION" -Raw).Trim()
}

if ([string]::IsNullOrWhiteSpace($Version)) {
  throw "VERSION is empty."
}

$ReleaseRoot = Join-Path $Root "release"
$PackageName = "HDU-Auto-Scheduling-Script-v$Version"
$PackageDir = Join-Path $ReleaseRoot $PackageName
$SamplesDir = Join-Path $PackageDir "samples"
$PackageScriptsDir = Join-Path $PackageDir "scripts"
$SmartAgentRoot = Join-Path $Root "HDU-Smart-Course-Agent"
$SmartAgentDir = Join-Path $PackageDir "smart-agent"
$SmartAgentDocsDir = Join-Path $SmartAgentDir "docs"

if (Test-Path -LiteralPath $PackageDir) {
  Remove-Item -LiteralPath $PackageDir -Recurse -Force
}

New-Item -ItemType Directory -Path $PackageDir | Out-Null
New-Item -ItemType Directory -Path $SamplesDir | Out-Null
New-Item -ItemType Directory -Path $PackageScriptsDir | Out-Null
New-Item -ItemType Directory -Path $SmartAgentDir | Out-Null
New-Item -ItemType Directory -Path $SmartAgentDocsDir | Out-Null

if (-not $SkipBuild) {
  Invoke-Native "go" @("test", "-buildvcs=false", "./...")
  Invoke-Native "go" @("build", "-buildvcs=false", "-ldflags", "-s -w", "-o", (Join-Path $PackageDir "HDU-Auto-Scheduling-Script.exe"), ".")

  Push-Location $SmartAgentRoot
  try {
    Invoke-Native "go" @("test", "-buildvcs=false", "./...")
    Invoke-Native "go" @("build", "-buildvcs=false", "-ldflags", "-s -w", "-o", (Join-Path $SmartAgentDir "HDU-Smart-Course-Agent.exe"), ".")
  } finally {
    Pop-Location
  }
}

if ($SkipBuild) {
  Copy-Item -LiteralPath (Join-Path $Root "dist\HDU-Auto-Scheduling-Script.exe") -Destination (Join-Path $PackageDir "HDU-Auto-Scheduling-Script.exe")
  Copy-Item -LiteralPath (Join-Path $SmartAgentRoot "HDU-Smart-Course-Agent.exe") -Destination (Join-Path $SmartAgentDir "HDU-Smart-Course-Agent.exe")
}

Copy-Item -LiteralPath (Join-Path $Root "VERSION") -Destination (Join-Path $PackageDir "VERSION.txt")
Copy-Item -LiteralPath (Join-Path $Root "docs\USER_GUIDE.md") -Destination (Join-Path $PackageDir "USER_GUIDE.md")
Copy-Item -LiteralPath (Join-Path $Root "docs\COURSE_SCHEMA.md") -Destination (Join-Path $PackageDir "COURSE_SCHEMA.md")
Copy-Item -LiteralPath (Join-Path $Root "docs\TEST_DATA.md") -Destination (Join-Path $PackageDir "TEST_DATA.md")
Copy-Item -LiteralPath (Join-Path $Root "README.md") -Destination (Join-Path $PackageDir "README.md")
Copy-Item -LiteralPath (Join-Path $Root "scripts\smart-agent-e2e.ps1") -Destination (Join-Path $PackageScriptsDir "smart-agent-e2e.ps1")
Copy-Item -LiteralPath (Join-Path $Root "scripts\smart-agent-ui-smoke.js") -Destination (Join-Path $PackageScriptsDir "smart-agent-ui-smoke.js")
Copy-Item -LiteralPath (Join-Path $Root "scripts\release-main-smoke.ps1") -Destination (Join-Path $PackageScriptsDir "release-main-smoke.ps1")
Copy-Item -LiteralPath (Join-Path $Root "testdata\course.sample.json") -Destination (Join-Path $SamplesDir "course.sample.json")
Copy-Item -LiteralPath (Join-Path $Root "testdata\personal-schedule.sample.json") -Destination (Join-Path $SamplesDir "personal-schedule.sample.json")
Copy-Item -LiteralPath (Join-Path $SmartAgentRoot "README.md") -Destination (Join-Path $SmartAgentDir "README.md")
Copy-Item -LiteralPath (Join-Path $SmartAgentRoot "SMART_AGENT_QUICKSTART.md") -Destination (Join-Path $SmartAgentDir "SMART_AGENT_QUICKSTART.md")
Copy-Item -LiteralPath (Join-Path $SmartAgentRoot "docs\EXECUTION_LOG_SCHEMA.md") -Destination (Join-Path $SmartAgentDocsDir "EXECUTION_LOG_SCHEMA.md")

$Manifest = [ordered]@{
  name = "HDU-Auto-Scheduling-Script"
  version = $Version
  builtAt = (Get-Date).ToString("s")
  files = @(
    "HDU-Auto-Scheduling-Script.exe",
    "VERSION.txt",
    "USER_GUIDE.md",
    "COURSE_SCHEMA.md",
    "TEST_DATA.md",
    "README.md",
    "scripts/smart-agent-e2e.ps1",
    "scripts/smart-agent-ui-smoke.js",
    "scripts/release-main-smoke.ps1",
    "samples/course.sample.json",
    "samples/personal-schedule.sample.json",
    "smart-agent/HDU-Smart-Course-Agent.exe",
    "smart-agent/README.md",
    "smart-agent/SMART_AGENT_QUICKSTART.md",
    "smart-agent/docs/EXECUTION_LOG_SCHEMA.md"
  )
}

$Manifest | ConvertTo-Json -Depth 4 | Set-Content -Path (Join-Path $PackageDir "manifest.json") -Encoding UTF8

# Generate SHA256SUMS.txt for every packaged file (relative forward-slash paths,
# sorted). The checksum file itself is intentionally not hashed.
$PackagedFiles = Get-ChildItem -LiteralPath $PackageDir -Recurse -File | Sort-Object FullName
$ChecksumLines = foreach ($file in $PackagedFiles) {
  $relative = $file.FullName.Substring($PackageDir.Length + 1).Replace('\', '/')
  $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLower()
  "$hash  $relative"
}
$ChecksumLines | Set-Content -Path (Join-Path $PackageDir "SHA256SUMS.txt") -Encoding ASCII

if (-not $NoZip) {
  $ZipPath = Join-Path $ReleaseRoot "$PackageName.zip"
  if (Test-Path -LiteralPath $ZipPath) {
    Remove-Item -LiteralPath $ZipPath -Force
  }
  Compress-Archive -Path (Join-Path $PackageDir "*") -DestinationPath $ZipPath
  $ZipHash = (Get-FileHash -LiteralPath $ZipPath -Algorithm SHA256).Hash.ToLower()
  "$ZipHash  $($PackageName).zip" | Set-Content -Path (Join-Path $ReleaseRoot "$($PackageName).zip.sha256") -Encoding ASCII
  Write-Host "Release package created: $ZipPath"
  Write-Host "Zip checksum written: $($ZipPath).sha256"
}

Write-Host "Release directory created: $PackageDir"
