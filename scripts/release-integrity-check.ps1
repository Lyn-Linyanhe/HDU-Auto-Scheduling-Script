param(
  [string]$ReleaseRoot = ""
)

$ErrorActionPreference = "Stop"
$ScriptRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ReleaseRoot)) {
  $ReleaseRoot = Join-Path $ScriptRoot "release"
}
$Version = (Get-Content -Path (Join-Path $ScriptRoot "VERSION") -Raw).Trim()
if ([string]::IsNullOrWhiteSpace($Version)) { throw "VERSION is empty." }
$PackageName = "HDU-Auto-Scheduling-Script-v$Version"
$ZipPath = Join-Path $ReleaseRoot "$PackageName.zip"
$ZipShaPath = "$ZipPath.sha256"
if (-not (Test-Path -LiteralPath $ZipPath)) { throw "ZIP not found: $ZipPath" }
if (-not (Test-Path -LiteralPath $ZipShaPath)) { throw "ZIP checksum not found: $ZipShaPath" }

# 1. The uploaded ZIP must match its sidecar checksum.
$ActualZipHash = (Get-FileHash -LiteralPath $ZipPath -Algorithm SHA256).Hash.ToLower()
$DeclaredZipHash = ((Get-Content -LiteralPath $ZipShaPath -Raw).Trim() -split '\s+')[0]
if ($ActualZipHash -ne $DeclaredZipHash) {
  throw "ZIP SHA256 mismatch: actual=$ActualZipHash declared=$DeclaredZipHash"
}

# 2. Extract and verify every manifest file against SHA256SUMS.txt.
$TempDir = Join-Path $env:TEMP "hdu-release-integrity-$PID"
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
try {
  Expand-Archive -LiteralPath $ZipPath -DestinationPath $TempDir -Force
  $Checksums = @{}
  foreach ($line in (Get-Content -LiteralPath (Join-Path $TempDir "SHA256SUMS.txt"))) {
    $parts = ($line.Trim() -split '\s+', 2)
    if ($parts.Count -ne 2 -or $parts[1].Length -eq 0) { continue }
    $Checksums[$parts[1].Replace('/', '\')] = $parts[0]
  }
  $Manifest = Get-Content -LiteralPath (Join-Path $TempDir "manifest.json") -Raw | ConvertFrom-Json
  if ($Manifest.version -ne $Version) {
    throw "Manifest version $($Manifest.version) does not match VERSION $Version."
  }
  foreach ($RelFile in $Manifest.files) {
    $RelWin = $RelFile.Replace('/', '\')
    if (-not $Checksums.ContainsKey($RelWin)) {
      throw "SHA256SUMS.txt missing entry for manifest file: $RelFile"
    }
    $FilePath = Join-Path $TempDir $RelWin
    if (-not (Test-Path -LiteralPath $FilePath)) {
      throw "Manifest file missing inside ZIP: $RelFile"
    }
    $Actual = (Get-FileHash -LiteralPath $FilePath -Algorithm SHA256).Hash.ToLower()
    if ($Actual -ne $Checksums[$RelWin]) {
      throw "SHA256 mismatch inside ZIP: $RelFile"
    }
  }
  if ($Checksums.Count -lt $Manifest.files.Count) {
    throw "SHA256SUMS.txt has fewer entries than the manifest."
  }
  Write-Host "Release integrity check passed. Verified v$Version ZIP and $($Manifest.files.Count) bundled files."
} finally {
  Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
