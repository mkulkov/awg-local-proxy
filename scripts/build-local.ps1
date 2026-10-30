#requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string] $CoreSource,
    [string] $GoExe = 'go'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$product = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$core = [IO.Path]::GetFullPath($CoreSource)
if (-not (Test-Path -LiteralPath (Join-Path $core 'go.mod') -PathType Leaf)) {
    throw 'CoreSource must be the separate core module root.'
}
$lock = Get-Content -Raw -LiteralPath (Join-Path $product 'core.lock.json') | ConvertFrom-Json
$sourceCommit = (& git -C $core rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $sourceCommit -ne $lock.source_commit) {
    throw 'Core source commit does not match core.lock.json.'
}
$sourceChanges = @(& git -C $core status --porcelain)
if ($LASTEXITCODE -ne 0 -or $sourceChanges.Count -ne 0) {
    throw 'Core source must be a clean checkout of the pinned commit.'
}
$stamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,8)
$destination = Join-Path $product ('dist\local-' + $stamp)
[void][IO.Directory]::CreateDirectory($destination)
$coreExe = Join-Path $destination 'awg-proxy-core.exe'
$guiExe = Join-Path $destination 'AwgLocalProxy.exe'

Push-Location $core
try {
    & $GoExe mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Core dependency verification failed.' }
    & $GoExe build -trimpath -buildvcs=false -o $coreExe ./cmd/awg-proxy-core
    if ($LASTEXITCODE -ne 0) { throw 'Core build failed.' }
} finally { Pop-Location }

$coreHash = (Get-FileHash -LiteralPath $coreExe -Algorithm SHA256).Hash.ToLowerInvariant()
Push-Location $product
try {
    & $GoExe mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Product dependency verification failed.' }
    & $GoExe build -trimpath -ldflags "-H=windowsgui -X main.expectedRealCoreSHA256=$coreHash" -o $guiExe ./cmd/awg-local-proxy
    if ($LASTEXITCODE -ne 0) { throw 'GUI build failed.' }
} finally { Pop-Location }

$guiHash = (Get-FileHash -LiteralPath $guiExe -Algorithm SHA256).Hash.ToLowerInvariant()
@("$guiHash  AwgLocalProxy.exe", "$coreHash  awg-proxy-core.exe") |
    Set-Content -LiteralPath (Join-Path $destination 'SHA256SUMS') -Encoding ascii
Copy-Item -LiteralPath (Join-Path $product 'README.md') -Destination $destination
Copy-Item -LiteralPath (Join-Path $product 'QUICKSTART.md') -Destination $destination
$lock.binary_sha256 = $coreHash
$lock.build_toolchain = (& $GoExe version).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Could not record the Go toolchain version.' }
$lock.binary_scope = 'unsigned local build; checksum describes the core included in this package'
$lock | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $destination 'core.lock.json')
Copy-Item -LiteralPath (Join-Path $product 'THIRD_PARTY_NOTICES.md') -Destination $destination
[void][IO.Directory]::CreateDirectory((Join-Path $destination 'docs'))
foreach ($name in @('CORE_SELECTION.md', 'LICENSE_AUDIT.md', 'IMPLEMENTATION.md', 'TEST_RESULTS.md', 'DEPENDENCY_LICENSE_REVIEW.md')) {
    Copy-Item -LiteralPath (Join-Path $product "docs\$name") -Destination (Join-Path $destination 'docs')
}
[void][IO.Directory]::CreateDirectory((Join-Path $destination 'third_party'))
Copy-Item -LiteralPath (Join-Path $product 'third_party\licenses') -Destination (Join-Path $destination 'third_party\licenses') -Recurse
$zip = "$destination.zip"
Compress-Archive -Path (Join-Path $destination '*') -DestinationPath $zip -CompressionLevel Optimal
Write-Host "Local build: $destination"
Write-Host "Local ZIP: $zip"
Write-Host 'Unsigned development build; a checksum does not authenticate its source. No release was published.'
