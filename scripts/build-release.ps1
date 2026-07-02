param(
  [string]$Version = "",
  [switch]$SkipBuild,
  [switch]$NoZip
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

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

if (Test-Path -LiteralPath $PackageDir) {
  Remove-Item -LiteralPath $PackageDir -Recurse -Force
}

New-Item -ItemType Directory -Path $PackageDir | Out-Null
New-Item -ItemType Directory -Path $SamplesDir | Out-Null

if (-not $SkipBuild) {
  go test -buildvcs=false ./...
  go build -buildvcs=false -ldflags "-s -w" -o (Join-Path $PackageDir "hdu-offline-scheduler.exe") .
  go build -buildvcs=false -ldflags "-s -w" -o (Join-Path $PackageDir "hdu-course-exporter.exe") ./cmd/course-exporter
}

if ($SkipBuild) {
  Copy-Item -LiteralPath (Join-Path $Root "dist\hdu-offline-scheduler.exe") -Destination (Join-Path $PackageDir "hdu-offline-scheduler.exe")
  Copy-Item -LiteralPath (Join-Path $Root "dist\hdu-course-exporter.exe") -Destination (Join-Path $PackageDir "hdu-course-exporter.exe")
}

Copy-Item -LiteralPath (Join-Path $Root "VERSION") -Destination (Join-Path $PackageDir "VERSION.txt")
Copy-Item -LiteralPath (Join-Path $Root "docs\USER_GUIDE.md") -Destination (Join-Path $PackageDir "USER_GUIDE.md")
Copy-Item -LiteralPath (Join-Path $Root "docs\COURSE_SCHEMA.md") -Destination (Join-Path $PackageDir "COURSE_SCHEMA.md")
Copy-Item -LiteralPath (Join-Path $Root "README.md") -Destination (Join-Path $PackageDir "README.md")
Copy-Item -LiteralPath (Join-Path $Root "testdata\course.sample.json") -Destination (Join-Path $SamplesDir "course.sample.json")
Copy-Item -LiteralPath (Join-Path $Root "testdata\personal-schedule.sample.json") -Destination (Join-Path $SamplesDir "personal-schedule.sample.json")

$Manifest = [ordered]@{
  name = "HDU-Auto-Scheduling-Script"
  version = $Version
  builtAt = (Get-Date).ToString("s")
  files = @(
    "hdu-offline-scheduler.exe",
    "hdu-course-exporter.exe",
    "VERSION.txt",
    "USER_GUIDE.md",
    "COURSE_SCHEMA.md",
    "README.md",
    "samples/course.sample.json",
    "samples/personal-schedule.sample.json"
  )
}

$Manifest | ConvertTo-Json -Depth 4 | Set-Content -Path (Join-Path $PackageDir "manifest.json") -Encoding UTF8

if (-not $NoZip) {
  $ZipPath = Join-Path $ReleaseRoot "$PackageName.zip"
  if (Test-Path -LiteralPath $ZipPath) {
    Remove-Item -LiteralPath $ZipPath -Force
  }
  Compress-Archive -Path (Join-Path $PackageDir "*") -DestinationPath $ZipPath
  Write-Host "Release package created: $ZipPath"
}

Write-Host "Release directory created: $PackageDir"
