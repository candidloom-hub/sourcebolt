<#
sourcebolt installer for Windows (Windows PowerShell 5.1 and PowerShell 7).

  Install or upgrade:
    irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex

  Uninstall (your sign-in data is kept; add -Purge to delete it too):
    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1))) -Uninstall

Options (when run as a script or through the scriptblock form above):
  -Version vX.Y.Z    install this release instead of the latest
  -InstallDir DIR    install into DIR instead of %LOCALAPPDATA%\Programs\sourcebolt
  -NoSetup           do not offer to run `sourcebolt setup` afterwards
  -NoModifyPath      do not add the install folder to your user PATH
  -Uninstall         remove sourcebolt.exe and its PATH entry
  -Purge             with -Uninstall: also delete your sourcebolt data (asks first)
  -Yes               answer "yes" to the -Purge question (for scripts)

Environment variables: SOURCEBOLT_VERSION, SOURCEBOLT_INSTALL_DIR, SOURCEBOLT_NO_SETUP=1,
SOURCEBOLT_DOWNLOAD_BASE (where release files come from; used by the tests, accepts file:// URLs),
SOURCEBOLT_NO_MODIFY_PATH=1 (same as -NoModifyPath; used by the tests).

What it does: downloads the release zip and checksums.txt, checks the SHA-256 of the zip (and the
cosign signature of checksums.txt when cosign is installed), copies sourcebolt.exe into your user
profile and adds that folder to your user PATH. No administrator rights are needed. Running it
again upgrades in place.
#>
param(
    [switch]$Uninstall,
    [switch]$Purge,
    [switch]$Yes,
    [switch]$NoSetup,
    [switch]$NoModifyPath,
    [string]$Version = $env:SOURCEBOLT_VERSION,
    [string]$InstallDir = $env:SOURCEBOLT_INSTALL_DIR
)

# Run as a script file (powershell -File install.ps1) we may set the process exit code; through
# `irm | iex` we must never call `exit`, because that would close the user's PowerShell window
# (or end the caller's script). Under iex, $PSCommandPath and $MyInvocation describe the caller,
# so only a script file that is itself install.ps1 counts.
$sourceboltRunAsFile = $false
try {
    $sourceboltRunAsFile = ($MyInvocation.MyCommand.CommandType -eq 'ExternalScript') -and
        ([IO.Path]::GetFileName($MyInvocation.MyCommand.Path) -ieq 'install.ps1')
} catch { }

$sourceboltState = @{ Failed = $false }
& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # the progress bar makes downloads much slower in 5.1
    Set-StrictMode -Version 2

    $repo = 'candidloom-hub/sourcebolt'
    $downloadBase = if ($env:SOURCEBOLT_DOWNLOAD_BASE) { $env:SOURCEBOLT_DOWNLOAD_BASE } else { "https://github.com/$repo/releases/download" }
    $dir = if ($InstallDir) { $InstallDir } else { Join-Path $env:LOCALAPPDATA 'Programs\sourcebolt' }
    $exe = Join-Path $dir 'sourcebolt.exe'
    $noSetup = $NoSetup -or ($env:SOURCEBOLT_NO_SETUP -eq '1')
    $touchPath = -not $NoModifyPath -and ($env:SOURCEBOLT_NO_MODIFY_PATH -ne '1')

    function Say([string]$m) { Write-Host $m }
    function Step([string]$m) { Write-Host "  - $m" }
    function Fail([string]$m) { throw [System.InvalidOperationException]::new($m) }

    function Test-Interactive {
        try { return [Environment]::UserInteractive -and -not [Console]::IsInputRedirected } catch { return $false }
    }

    # Returns $false (never the default) when nobody can answer: Read-Host throws in hosts that
    # cannot prompt, such as -NonInteractive sessions.
    function Confirm-Yes([string]$question, [bool]$defaultYes) {
        if (-not (Test-Interactive)) { return $false }
        $hint = if ($defaultYes) { '[Y/n]' } else { '[y/N]' }
        try { $a = Read-Host "$question $hint" } catch { return $false }
        if ([string]::IsNullOrWhiteSpace($a)) { return $defaultYes }
        return $a.Trim() -match '^(y|yes)$'
    }

    # The URL scheme a download may use: https, or file for the installer tests.
    function Assert-AllowedUrl([string]$url) {
        if ($url -notmatch '^(https|file)://') { Fail "downloads must use HTTPS (got '$url'). Only HTTPS downloads are allowed." }
    }

    function Get-FullDir([string]$p) {
        try { return [IO.Path]::GetFullPath($p).TrimEnd('\', '.').TrimEnd('\') } catch { return '' }
    }

    # Refuses to recursively delete a folder that is clearly not sourcebolt's own: a drive root,
    # or any of the profile, AppData and system folders, or a folder above one of them. Both
    # sides are compared as full paths, so 'C:\Users\me\' and 'C:\Users\me\.' match too.
    function Assert-SafeToPurge([string]$p) {
        $full = Get-FullDir $p
        $isRoot = $full -and ([IO.Path]::GetPathRoot($full + '\').TrimEnd('\') -ieq $full)
        if (-not $full -or $isRoot) { Fail "refusing to delete '$p': it is a drive root or not a full path. Check NOTEBOOKLM_CONFIG_DIR." }
        $danger = @($env:USERPROFILE, $env:APPDATA, $env:LOCALAPPDATA, $env:HOME, $env:SystemRoot, $env:ProgramFiles, ${env:ProgramFiles(x86)}) |
            Where-Object { $_ } | ForEach-Object { Get-FullDir $_ } | Where-Object { $_ }
        foreach ($d in $danger) {
            if ($d -ieq $full -or $d.StartsWith($full + '\', [StringComparison]::OrdinalIgnoreCase)) {
                Fail "refusing to delete '$p': it is your profile, AppData or a system folder, or a folder above one of them. Check NOTEBOOKLM_CONFIG_DIR."
            }
        }
    }

    # Runs a native program without letting its stderr turn into a terminating error (5.1).
    function Invoke-Native([string]$file, [string[]]$arguments) {
        $old = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            $out = & $file @arguments 2>&1 | ForEach-Object { "$_" }
            return [pscustomobject]@{ Code = $LASTEXITCODE; Output = ($out -join "`n") }
        } catch {
            return [pscustomobject]@{ Code = 1; Output = "$_" }
        } finally {
            $ErrorActionPreference = $old
        }
    }

    function Get-Download([string]$url, [string]$dest) {
        if ($url -like 'file://*') {
            Copy-Item -LiteralPath ([Uri]$url).LocalPath -Destination $dest -Force
            return
        }
        Assert-AllowedUrl $url
        $r = Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing -PassThru
        # Invoke-WebRequest follows redirects on its own; refuse the file if one went to plain HTTP.
        $final = $null
        if ($r.BaseResponse.PSObject.Properties['ResponseUri']) { $final = $r.BaseResponse.ResponseUri }
        elseif ($r.BaseResponse.PSObject.Properties['RequestMessage']) { $final = $r.BaseResponse.RequestMessage.RequestUri }
        if ($final -and $final.Scheme -ne 'https') {
            Remove-Item -LiteralPath $dest -Force -ErrorAction SilentlyContinue
            Fail "a download was redirected to $($final.Scheme)://; only HTTPS downloads are allowed."
        }
    }

    function Get-ConfigDir {
        if ($env:NOTEBOOKLM_CONFIG_DIR) { return $env:NOTEBOOKLM_CONFIG_DIR }
        return Join-Path $env:APPDATA 'sourcebolt'
    }

    # User PATH, read and written raw so %VARIABLES% in it stay unexpanded.
    function Get-UserPath {
        $k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
        try { if ($k) { return [string]$k.GetValue('Path', '', 'DoNotExpandEnvironmentNames') } else { return '' } } finally { if ($k) { $k.Close() } }
    }
    function Set-UserPath([string]$value) {
        $k = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey('Environment')
        try {
            # Keep the value's existing kind (REG_SZ or REG_EXPAND_SZ); new values get ExpandString.
            $kind = [Microsoft.Win32.RegistryValueKind]::ExpandString
            if ($k.GetValueNames() -contains 'Path') { $kind = $k.GetValueKind('Path') }
            $k.SetValue('Path', $value, $kind)
        } finally { $k.Close() }
        # Tell Explorer and new terminals that the environment changed (WM_SETTINGCHANGE).
        [Environment]::SetEnvironmentVariable('SOURCEBOLT_INSTALLER_REFRESH', '1', 'User')
        [Environment]::SetEnvironmentVariable('SOURCEBOLT_INSTALLER_REFRESH', $null, 'User')
    }
    function Test-SameDir([string]$a, [string]$b) {
        $x = [Environment]::ExpandEnvironmentVariables($a).Trim().TrimEnd('\')
        $y = [Environment]::ExpandEnvironmentVariables($b).Trim().TrimEnd('\')
        return [string]::Equals($x, $y, [StringComparison]::OrdinalIgnoreCase)
    }

    try {
        try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }

        # The folder goes into the user PATH, where ';' separates entries and '"' is not allowed.
        $dirOk = $false
        # Only a full path: 'C:\x' or '\\server\share\x', not drive-relative 'C:x' or '\x'.
        try { $dirOk = ($dir -match '^([A-Za-z]:\\|\\\\[^\\]+\\[^\\]+\\)') -and ($dir -notmatch '[;"]') -and ([IO.Path]::GetPathRoot($dir).TrimEnd('\') -ine $dir.TrimEnd('\')) } catch { }
        if (-not $dirOk) { Fail "'$dir' cannot be used as the install folder. Use a full path of its own, with no ';' or '`"'." }

        # ---------------------------------------------------------------- uninstall
        if ($Uninstall) {
            Say 'Removing sourcebolt'
            if (Test-Path -LiteralPath $exe) {
                try { Remove-Item -LiteralPath $exe -Force } catch { Fail "could not delete $exe. Close any program that is using sourcebolt (for example your AI app) and try again." }
                Step "removed $exe"
            } else {
                Step "no sourcebolt.exe in $dir (nothing to remove)"
            }
            Get-ChildItem -LiteralPath $dir -Filter 'sourcebolt.exe.old*' -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
            if ((Test-Path -LiteralPath $dir) -and -not (Get-ChildItem -LiteralPath $dir -Force)) {
                Remove-Item -LiteralPath $dir -Force
                Step "removed the empty folder $dir"
            }
            if ($touchPath) {
                $parts = @((Get-UserPath) -split ';' | Where-Object { $_ -ne '' })
                $kept = @($parts | Where-Object { -not (Test-SameDir $_ $dir) })
                if ($kept.Count -ne $parts.Count) {
                    Set-UserPath ($kept -join ';')
                    Step "removed $dir from your user PATH"
                } else {
                    Step 'your user PATH did not contain the sourcebolt folder'
                }
            }
            $data = Get-ConfigDir
            if ($Purge) {
                if (-not (Test-Path -LiteralPath $data)) {
                    Step "no sourcebolt data at $data"
                } else {
                    Assert-SafeToPurge $data
                    if ($Yes -or (Confirm-Yes "Also delete your sourcebolt sign-in data at ${data}? This signs you out for good." $false)) {
                        Remove-Item -LiteralPath $data -Recurse -Force
                        Step "deleted $data"
                    } else {
                        Step "kept your sourcebolt data at $data (run again with -Purge -Yes to delete it)"
                    }
                }
            } elseif (Test-Path -LiteralPath $data) {
                Step "kept your sourcebolt data at $data (add -Purge to delete it)"
            }
            Say 'Done.'
            return
        }

        # ---------------------------------------------------------------- platform
        $osArch = $null
        try { $osArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString() } catch { }
        if (-not $osArch) { $osArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE } }
        switch -Regex ($osArch) {
            '^(X64|AMD64)$' { $arch = 'amd64'; break }
            '^ARM64$' { $arch = 'arm64'; break }
            default { Fail "unsupported processor '$osArch'. Builds exist for x64 and ARM64 only." }
        }

        # ---------------------------------------------------------------- version
        $ver = $Version
        if (-not $ver) {
            try {
                $ver = (Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing -Headers @{ 'User-Agent' = 'sourcebolt-installer' }).tag_name
            } catch { $ver = $null }
            if (-not $ver) {
                # API rate-limited? /releases/latest redirects to /releases/tag/<tag>.
                try {
                    $r = Invoke-WebRequest -Uri "https://github.com/$repo/releases/latest" -UseBasicParsing
                    $final = $null
                    if ($r.BaseResponse.PSObject.Properties['ResponseUri']) { $final = $r.BaseResponse.ResponseUri.AbsoluteUri }
                    elseif ($r.BaseResponse.PSObject.Properties['RequestMessage']) { $final = $r.BaseResponse.RequestMessage.RequestUri.AbsoluteUri }
                    if ($final -match '/tag/([^/?#]+)') { $ver = $Matches[1] }
                } catch { $ver = $null }
            }
            if (-not $ver) { Fail 'could not find the latest release. Check your internet connection, or set $env:SOURCEBOLT_VERSION = "v0.2.0" and try again.' }
        }
        if (-not $ver.StartsWith('v')) { $ver = "v$ver" }
        if ($ver -cnotmatch '^v\d+\.\d+\.\d+(-[A-Za-z0-9_.]+)?$') { Fail "'$ver' is not a valid version. Use a release tag such as v0.2.0." }
        Assert-AllowedUrl $downloadBase
        $num = $ver.Substring(1)
        $archive = "sourcebolt_${num}_windows_${arch}.zip"
        $base = "$($downloadBase.TrimEnd('/'))/$ver"

        Say "Installing sourcebolt $ver for windows/$arch"

        $tmp = Join-Path ([IO.Path]::GetTempPath()) ("sourcebolt-install-" + [Guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $tmp | Out-Null
        try {
            # ------------------------------------------------------------ download and verify
            Step "downloading $archive"
            try { Get-Download "$base/$archive" (Join-Path $tmp $archive) } catch { Fail "could not download $base/$archive ($($_.Exception.Message))" }
            try { Get-Download "$base/checksums.txt" (Join-Path $tmp 'checksums.txt') } catch { Fail "could not download $base/checksums.txt ($($_.Exception.Message))" }

            $cosign = Get-Command cosign -ErrorAction SilentlyContinue
            if ($cosign) {
                Step 'cosign found: checking the signature of checksums.txt'
                try { Get-Download "$base/checksums.txt.sigstore.json" (Join-Path $tmp 'checksums.txt.sigstore.json') } catch { Fail 'cosign is installed but the signature file could not be downloaded.' }
                $v = Invoke-Native $cosign.Source @('verify-blob', (Join-Path $tmp 'checksums.txt'),
                    '--bundle', (Join-Path $tmp 'checksums.txt.sigstore.json'),
                    '--certificate-identity-regexp', "^https://github.com/$repo/\.github/workflows/release\.yml@refs/tags/v",
                    '--certificate-oidc-issuer', 'https://token.actions.githubusercontent.com')
                if ($v.Code -ne 0) { Fail 'the signature of checksums.txt did not verify. The download may have been tampered with.' }
                Step 'signature verified (cosign)'
            } else {
                Step 'cosign not installed: signature check skipped. The SHA-256 check below is integrity only: it proves the file matches checksums.txt, not who published it.'
            }

            $expected = $null
            foreach ($line in Get-Content -LiteralPath (Join-Path $tmp 'checksums.txt')) {
                $f = $line -split '\s+', 2
                if ($f.Count -eq 2 -and ($f[1].Trim() -eq $archive -or $f[1].Trim() -eq "*$archive")) { $expected = $f[0].ToLowerInvariant(); break }
            }
            if (-not $expected) { Fail "checksums.txt has no entry for $archive." }
            $actual = (Get-FileHash -LiteralPath (Join-Path $tmp $archive) -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($actual -ne $expected) { Fail "checksum mismatch for $archive (expected $expected, got $actual). The download may be corrupt or tampered with; nothing was changed." }
            Step 'SHA-256 checksum verified'

            # ------------------------------------------------------------ install
            $x = Join-Path $tmp 'x'
            Expand-Archive -LiteralPath (Join-Path $tmp $archive) -DestinationPath $x -Force
            $newExe = Join-Path $x 'sourcebolt.exe'
            if (-not (Test-Path -LiteralPath $newExe)) { Fail "$archive does not contain sourcebolt.exe." }

            New-Item -ItemType Directory -Force -Path $dir | Out-Null
            Get-ChildItem -LiteralPath $dir -Filter 'sourcebolt.exe.old*' -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
            $hadOld = Test-Path -LiteralPath $exe
            if ($hadOld) {
                # A running .exe cannot be overwritten but can be renamed.
                $old = "$exe.old-" + [Guid]::NewGuid().ToString('N').Substring(0, 8)
                try { Move-Item -LiteralPath $exe -Destination $old -Force } catch { Fail "could not replace $exe. Close any program that is using sourcebolt and try again." }
            }
            # Keep the old copy until the new one has run, so a broken upgrade can be undone.
            $restore = {
                Remove-Item -LiteralPath $exe -Force -ErrorAction SilentlyContinue
                if ($hadOld) {
                    try { Move-Item -LiteralPath $old -Destination $exe -Force } catch {
                        Fail "the new program did not install, and the previous version could not be put back. It is saved as $old; rename it to sourcebolt.exe."
                    }
                }
            }
            try { Copy-Item -LiteralPath $newExe -Destination $exe -Force } catch {
                & $restore
                Fail "could not write $exe ($($_.Exception.Message)); the previous version was kept."
            }
            $vr = Invoke-Native $exe @('--version')
            if ($vr.Code -ne 0) {
                & $restore
                if ($hadOld) { Fail "the new program did not run ($exe --version); restored the previous version." }
                Fail "the installed program did not run: $exe --version"
            }
            if ($hadOld) {
                Remove-Item -LiteralPath $old -Force -ErrorAction SilentlyContinue
                Step "upgraded $exe"
            } else {
                Step "installed $exe"
            }
        } finally {
            Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
        }

        Say ''
        Say "Installed: $($vr.Output.Trim())"

        # ---------------------------------------------------------------- PATH
        if ($touchPath) {
            $userPath = Get-UserPath
            $already = @($userPath -split ';' | Where-Object { $_ -ne '' -and (Test-SameDir $_ $dir) }).Count -gt 0
            if (-not $already) {
                $newPath = if ($userPath) { $userPath.TrimEnd(';') + ';' + $dir } else { $dir }
                Set-UserPath $newPath
                Step "added $dir to your user PATH (open a new terminal to use 'sourcebolt')"
            }
            if (-not (@($env:Path -split ';' | Where-Object { $_ -ne '' -and (Test-SameDir $_ $dir) }).Count -gt 0)) {
                $env:Path = $env:Path.TrimEnd(';') + ';' + $dir # this window too
            }
        }

        # ---------------------------------------------------------------- next step
        Say ''
        $hasSetup = (Invoke-Native $exe @('setup', '--help')).Code -eq 0
        if ($hasSetup) {
            if ($noSetup) {
                Say "Next step: run 'sourcebolt setup' to sign in and connect your AI apps."
            } elseif (Confirm-Yes "Run 'sourcebolt setup' now to sign in and connect your AI apps?" $true) {
                # Start-Process keeps setup on the real console, so its questions show at once.
                $p = Start-Process -FilePath $exe -ArgumentList 'setup' -NoNewWindow -Wait -PassThru
                if ($p.ExitCode -ne 0) { Say "Setup did not finish. You can run 'sourcebolt setup' again at any time." }
            } else {
                Say "Next step: run 'sourcebolt setup' to sign in and connect your AI apps."
            }
        } else {
            Say 'Next step: sign in to NotebookLM. run: sourcebolt login'
            Say "Guide: https://github.com/$repo/blob/main/docs/auth.md"
        }
        Say "To uninstall later: & ([scriptblock]::Create((irm https://raw.githubusercontent.com/$repo/main/install.ps1))) -Uninstall"
        return
    } catch {
        Write-Host ''
        Write-Host "sourcebolt was NOT installed: $($_.Exception.Message)" -ForegroundColor Red
        $sourceboltState.Failed = $true
    }
}

if (-not $sourceboltState.Failed) {
    $global:LASTEXITCODE = 0
} else {
    if ($sourceboltRunAsFile) { exit 1 }
    $global:LASTEXITCODE = 1
}
