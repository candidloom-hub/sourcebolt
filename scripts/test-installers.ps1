<#
Offline tests for install.ps1. They build a fake release (a real sourcebolt.exe built with
`go build` and a made-up version) and point the installer at it with
SOURCEBOLT_DOWNLOAD_BASE=file:///..., so nothing is downloaded.

  powershell -NoProfile -File scripts\test-installers.ps1                 # Windows PowerShell 5.1
  pwsh -NoProfile -File scripts\test-installers.ps1 -Shell pwsh           # PowerShell 7

The user PATH is left alone (SOURCEBOLT_NO_MODIFY_PATH=1) unless -TestUserPath is given; CI
passes it on its throwaway runner to check that install adds and uninstall removes the entry.
#>
param(
    [ValidateSet('powershell', 'pwsh')][string]$Shell = 'powershell',
    [switch]$TestUserPath
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$installer = Join-Path $root 'install.ps1'
$work = Join-Path ([IO.Path]::GetTempPath()) ('sb-installer-test-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work | Out-Null
$script:pass = 0
$script:fail = 0
$script:out = ''

function Ok([string]$name) { $script:pass++; Write-Host "ok   $name" }
function Bad([string]$name) {
    $script:fail++
    Write-Host "FAIL $name"
    $script:out -split "`n" | ForEach-Object { Write-Host "     | $_" }
}

$arch = 'amd64'
try { if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString() -eq 'Arm64') { $arch = 'arm64' } } catch { }

function New-Release([string]$v) {
    $d = Join-Path $work "rel\v$v"
    $pkg = Join-Path $work "pkg-$v"
    New-Item -ItemType Directory -Force -Path $d, $pkg | Out-Null
    $env:CGO_ENABLED = '0'
    & go build -trimpath -ldflags "-s -w -X github.com/candidloom-hub/sourcebolt/internal/version.Version=$v" -o (Join-Path $pkg 'sourcebolt.exe') (Join-Path $root 'cmd\sourcebolt')
    if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
    $zip = Join-Path $d "sourcebolt_${v}_windows_${arch}.zip"
    Compress-Archive -Path (Join-Path $pkg 'sourcebolt.exe') -DestinationPath $zip -Force
    $h = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $d 'checksums.txt'), "$h  sourcebolt_${v}_windows_${arch}.zip`n")
}

$bin = Join-Path $work 'bin'
$exe = Join-Path $bin 'sourcebolt.exe'
$config = Join-Path $work 'config'

$script:baseOverride = $null
$script:envOverride = $null

function Invoke-Installer([string[]]$extra) {
    $env:SOURCEBOLT_DOWNLOAD_BASE = if ($script:baseOverride) { $script:baseOverride } else { 'file:///' + ((Join-Path $work 'rel') -replace '\\', '/') }
    $env:SOURCEBOLT_INSTALL_DIR = $bin
    $env:SOURCEBOLT_NO_SETUP = '1'
    $env:NOTEBOOKLM_CONFIG_DIR = $config
    if ($TestUserPath) { Remove-Item Env:SOURCEBOLT_NO_MODIFY_PATH -ErrorAction SilentlyContinue } else { $env:SOURCEBOLT_NO_MODIFY_PATH = '1' }
    $saved = @{}
    if ($script:envOverride) {
        foreach ($n in $script:envOverride.Keys) {
            $saved[$n] = [Environment]::GetEnvironmentVariable($n, 'Process')
            [Environment]::SetEnvironmentVariable($n, $script:envOverride[$n], 'Process')
        }
    }
    $old = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $o = & $Shell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $installer @extra 2>&1 | ForEach-Object { "$_" }
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $old
        foreach ($n in $saved.Keys) { [Environment]::SetEnvironmentVariable($n, $saved[$n], 'Process') }
    }
    $script:out = $o -join "`n"
    return $code
}

function Get-Ver {
    if (-not (Test-Path -LiteralPath $exe)) { return '' }
    try { return (& $exe --version) -join '' } catch { return 'does not run' }
}

function Test-UserPathHas([string]$d) {
    $k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
    $p = if ($k) { [string]$k.GetValue('Path', '', 'DoNotExpandEnvironmentNames') } else { '' }
    return @($p -split ';' | Where-Object { $_.TrimEnd('\') -ieq $d.TrimEnd('\') }).Count -gt 0
}

try {
    New-Release '1.0.0'
    New-Release '1.1.0'

    # 1. good install
    if ((Invoke-Installer @('-Version', 'v1.0.0')) -eq 0 -and (Get-Ver) -eq 'sourcebolt 1.0.0' -and $script:out -match 'SHA-256 checksum verified') {
        Ok 'installs and verifies the checksum'
    } else { Bad 'installs and verifies the checksum' }

    if ($TestUserPath) {
        if (Test-UserPathHas $bin) { Ok 'adds the folder to the user PATH' } else { Bad 'adds the folder to the user PATH' }
    }

    # 2. upgrade and re-run
    if ((Invoke-Installer @('-Version', '1.1.0')) -eq 0 -and (Get-Ver) -eq 'sourcebolt 1.1.0' -and $script:out -match 'upgraded' -and
        (Invoke-Installer @('-Version', 'v1.1.0')) -eq 0) {
        Ok 'upgrades in place and re-runs cleanly'
    } else { Bad 'upgrades in place and re-runs cleanly' }

    if ($TestUserPath) {
        $k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
        $n = @(([string]$k.GetValue('Path', '', 'DoNotExpandEnvironmentNames')) -split ';' | Where-Object { $_.TrimEnd('\') -ieq $bin.TrimEnd('\') }).Count
        $k.Close()
        if ($n -eq 1) { Ok 'does not add the PATH entry twice' } else { Bad "does not add the PATH entry twice (found $n)" }
    }

    # 3. tampered checksums.txt: abort, keep the installed binary
    $sums = Join-Path $work 'rel\v1.0.0\checksums.txt'
    $good = [IO.File]::ReadAllText($sums)
    $first = $good.Substring(0, 1)
    $tampered = $(if ($first -eq 'a') { 'b' } else { 'a' }) + $good.Substring(1)
    [IO.File]::WriteAllText($sums, $tampered)
    if ((Invoke-Installer @('-Version', 'v1.0.0')) -ne 0 -and $script:out -match 'checksum mismatch' -and (Get-Ver) -eq 'sourcebolt 1.1.0') {
        Ok 'aborts on a tampered checksums.txt and leaves the old binary'
    } else { Bad 'aborts on a tampered checksums.txt and leaves the old binary' }

    # 4. tampered archive
    [IO.File]::WriteAllText($sums, $good)
    $zip = Join-Path $work "rel\v1.0.0\sourcebolt_1.0.0_windows_${arch}.zip"
    $zipBytes = [IO.File]::ReadAllBytes($zip)
    [IO.File]::WriteAllBytes($zip, $zipBytes + [byte[]](0x78))
    if ((Invoke-Installer @('-Version', 'v1.0.0')) -ne 0 -and $script:out -match 'checksum mismatch') {
        Ok 'aborts on a tampered archive'
    } else { Bad 'aborts on a tampered archive' }
    [IO.File]::WriteAllBytes($zip, $zipBytes)

    # 5. no entry in checksums.txt
    [IO.File]::WriteAllText($sums, '')
    if ((Invoke-Installer @('-Version', 'v1.0.0')) -ne 0 -and $script:out -match 'no entry') {
        Ok 'aborts when checksums.txt has no entry'
    } else { Bad 'aborts when checksums.txt has no entry' }
    [IO.File]::WriteAllText($sums, $good)

    # 6. uninstall keeps data (and removes the PATH entry)
    New-Item -ItemType Directory -Force -Path $config | Out-Null
    Set-Content -LiteralPath (Join-Path $config 'profile') -Value keep
    if ((Invoke-Installer @('-Uninstall')) -eq 0 -and -not (Test-Path -LiteralPath $exe) -and (Test-Path -LiteralPath (Join-Path $config 'profile'))) {
        Ok 'uninstall removes the binary and keeps the data'
    } else { Bad 'uninstall removes the binary and keeps the data' }
    if ($TestUserPath) {
        if (-not (Test-UserPathHas $bin)) { Ok 'uninstall removes the user PATH entry' } else { Bad 'uninstall removes the user PATH entry' }
    }

    # 7. -Purge without confirmation keeps data
    if ((Invoke-Installer @('-Uninstall', '-Purge')) -eq 0 -and (Test-Path -LiteralPath (Join-Path $config 'profile'))) {
        Ok '-Purge without confirmation keeps the data'
    } else { Bad '-Purge without confirmation keeps the data' }

    # 8. -Purge -Yes deletes data
    if ((Invoke-Installer @('-Uninstall', '-Purge', '-Yes')) -eq 0 -and -not (Test-Path -LiteralPath $config)) {
        Ok '-Purge -Yes deletes the data'
    } else { Bad '-Purge -Yes deletes the data' }

    # --- hardening (security review) ---------------------------------------------------

    # 9. malformed version
    if ((Invoke-Installer @('-Version', 'v1.0.0;calc')) -ne 0 -and $script:out -match 'not a valid version' -and -not (Test-Path -LiteralPath $exe)) {
        Ok 'refuses a malformed version'
    } else { Bad 'refuses a malformed version' }

    # 10. plain-http download base
    $savedBase = $null
    $script:baseOverride = 'http://example.invalid/rel'
    if ((Invoke-Installer @('-Version', 'v1.0.0')) -ne 0 -and $script:out -match 'HTTPS' -and -not (Test-Path -LiteralPath $exe)) {
        Ok 'refuses a non-HTTPS download base'
    } else { Bad 'refuses a non-HTTPS download base' }
    $script:baseOverride = $null

    # 11. an install folder with ';' would split the PATH entry
    if ((Invoke-Installer @('-Version', 'v1.0.0', '-InstallDir', "$work\a;b")) -ne 0 -and $script:out -match 'cannot be used as the install folder') {
        Ok "refuses an install folder with ';'"
    } else { Bad "refuses an install folder with ';'" }

    # 12. -Purge never deletes the user profile folder (USERPROFILE is a temp folder here)
    $fakeProfile = Join-Path $work 'profile-home'
    New-Item -ItemType Directory -Force -Path $fakeProfile | Out-Null
    Set-Content -LiteralPath (Join-Path $fakeProfile 'keep-me') -Value keep
    $script:envOverride = @{ USERPROFILE = $fakeProfile; NOTEBOOKLM_CONFIG_DIR = $fakeProfile }
    $code = Invoke-Installer @('-Uninstall', '-Purge', '-Yes')
    $script:envOverride = $null
    if ($code -ne 0 -and (Test-Path -LiteralPath (Join-Path $fakeProfile 'keep-me')) -and $script:out -match 'refusing to delete') {
        Ok 'refuses to purge the user profile folder'
    } else { Bad 'refuses to purge the user profile folder' }

    # 12b. ... nor a folder above it, nor a spelling of it with a trailing '\' or '\.'
    foreach ($target in @((Split-Path -Parent $fakeProfile), "$fakeProfile\", "$fakeProfile\.")) {
        $script:envOverride = @{ USERPROFILE = $fakeProfile; NOTEBOOKLM_CONFIG_DIR = $target }
        $code = Invoke-Installer @('-Uninstall', '-Purge', '-Yes')
        $script:envOverride = $null
        if ($code -ne 0 -and (Test-Path -LiteralPath (Join-Path $fakeProfile 'keep-me')) -and $script:out -match 'refusing to delete') {
            Ok "refuses to purge '$(Split-Path -Leaf $target)' (profile spelling or parent)"
        } else { Bad "refuses to purge '$target'" }
    }

    # 12c. a drive-relative install folder is refused
    if ((Invoke-Installer @('-Version', 'v1.0.0', '-InstallDir', 'C:relative\sb')) -ne 0 -and $script:out -match 'cannot be used as the install folder') {
        Ok 'refuses a drive-relative install folder'
    } else { Bad 'refuses a drive-relative install folder' }

    # 12d. through iex inside another script, a failure sets $LASTEXITCODE and never exits the caller
    $caller = Join-Path $work 'caller.ps1'
    [IO.File]::WriteAllText($caller, ('$env:SOURCEBOLT_DOWNLOAD_BASE = "http://example.invalid"' + "`n" +
            "Get-Content -Raw -LiteralPath '$installer' | Invoke-Expression`n" + '"caller still running: $LASTEXITCODE"' + "`n"))
    $old = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
    $o = (& $Shell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $caller 2>&1 | ForEach-Object { "$_" }) -join "`n"
    $ErrorActionPreference = $old
    $script:out = $o
    if ($o -match 'caller still running: 1') { Ok 'iex inside a script reports failure without exiting the caller' } else { Bad 'iex inside a script reports failure without exiting the caller' }

    # 13. without cosign the output says the check is integrity only
    if ((Invoke-Installer @('-Version', 'v1.0.0')) -eq 0 -and $script:out -match 'integrity only') {
        Ok "says 'integrity only' when cosign is absent"
    } else { Bad "says 'integrity only' when cosign is absent" }

    # 14. a new binary that does not run is rolled back to the old one
    $d = Join-Path $work 'rel\v1.2.0'
    $pkg = Join-Path $work 'pkg-broken'
    New-Item -ItemType Directory -Force -Path $d, $pkg | Out-Null
    [IO.File]::WriteAllText((Join-Path $pkg 'sourcebolt.exe'), 'not a program')
    $bz = Join-Path $d "sourcebolt_1.2.0_windows_${arch}.zip"
    Compress-Archive -Path (Join-Path $pkg 'sourcebolt.exe') -DestinationPath $bz -Force
    $bh = (Get-FileHash -LiteralPath $bz -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $d 'checksums.txt'), "$bh  sourcebolt_1.2.0_windows_${arch}.zip`n")
    if ((Invoke-Installer @('-Version', 'v1.2.0')) -ne 0 -and (Get-Ver) -eq 'sourcebolt 1.0.0' -and $script:out -match 'restored') {
        Ok 'rolls back a broken upgrade'
    } else { Bad 'rolls back a broken upgrade' }
} finally {
    Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host "$($script:pass) passed, $($script:fail) failed ($Shell)"
if ($script:fail -ne 0) { exit 1 }
