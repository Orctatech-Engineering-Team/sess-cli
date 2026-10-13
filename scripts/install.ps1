#requires -Version 5.1
[CmdletBinding()]
param(
    [string]$InstallDir = $env:SESS_INSTALL_DIR,
    [string]$Version = $env:SESS_VERSION,
    [string]$BaseUrl = $env:SESS_BASE_URL,
    [string]$Repository = $env:SESS_REPO
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Get-SessArchitecture {
    $architecture = $env:PROCESSOR_ARCHITEW6432
    if (-not $architecture) { $architecture = $env:PROCESSOR_ARCHITECTURE }
    switch ($architecture) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { throw "Unsupported Windows architecture: $architecture" }
    }
}

function Get-SessApplication {
    $commands = @(Get-Command sess.exe -CommandType Application -ErrorAction SilentlyContinue)
    if ($commands.Count -gt 0) { return $commands[0].Path }
    return $null
}

function Assert-SessTarget {
    param([string]$Directory, [string]$Target)
    $current = $Directory
    while ($current) {
        if (Test-Path -LiteralPath $current) {
            $item = Get-Item -LiteralPath $current -Force
            if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
                throw "Install path uses a symlink or junction: $current. Use its package manager or choose another InstallDir."
            }
        }
        $current = [IO.Path]::GetDirectoryName($current)
    }
    if (Test-Path -LiteralPath $Target) {
        $item = Get-Item -LiteralPath $Target -Force
        if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw "$Target is not a regular file or is a symlink. Use its package manager or choose another InstallDir."
        }
        if ($item.Attributes -band [IO.FileAttributes]::ReadOnly) {
            throw "$Target is read-only. Use its package manager or choose a writable InstallDir."
        }
        # A running executable or another exclusive user must block replacement.
        $handle = [IO.File]::Open($Target, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
        $handle.Dispose()
    }
}

function Save-SessDownload {
    param([string]$Url, [string]$Destination)
    $uri = [Uri]$Url
    if ($uri.IsFile) {
        [IO.File]::Copy($uri.LocalPath, $Destination)
    } else {
        if ($uri.Scheme -ne 'https') { throw 'Downloads require HTTPS or a local file URL.' }
        Invoke-WebRequest -UseBasicParsing -Uri $uri -OutFile $Destination
    }
}

function Install-Sess {
    if ($env:OS -ne 'Windows_NT') { throw 'This installer requires Windows.' }
    if (-not $Version) { $Version = 'latest' }
    if (-not $Repository) { $Repository = 'Orctatech-Engineering-Team/sess-cli' }
    $archiveName = "sess-windows-$(Get-SessArchitecture).zip"
    if (-not $BaseUrl) {
        if ($Version -eq 'latest') {
            $BaseUrl = "https://github.com/$Repository/releases/latest/download"
        } else {
            $BaseUrl = "https://github.com/$Repository/releases/download/$Version"
        }
    }
    if (-not $InstallDir) {
        $existing = Get-SessApplication
        if ($existing) {
            $InstallDir = [IO.Path]::GetDirectoryName($existing)
            Write-Host "Found existing installation: $existing"
        } else {
            if (-not $env:LOCALAPPDATA) { throw 'LOCALAPPDATA is not set; choose an InstallDir.' }
            $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\SESS'
        }
    }
    $InstallDir = [IO.Path]::GetFullPath($InstallDir)
    $target = Join-Path $InstallDir 'sess.exe'
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("sess-install-" + [Guid]::NewGuid().ToString('N'))
    $staged = $null
    $oldTls = [Net.ServicePointManager]::SecurityProtocol
    try {
        Assert-SessTarget $InstallDir $target
        [IO.Directory]::CreateDirectory($InstallDir) | Out-Null
        $staged = Join-Path $InstallDir ('.sess-install-' + [Guid]::NewGuid().ToString('N'))
        # Probe write permissions before downloading.
        $probe = [IO.File]::Create($staged)
        $probe.Dispose()
        [IO.Directory]::CreateDirectory($temporary) | Out-Null
        [Net.ServicePointManager]::SecurityProtocol = $oldTls -bor [Net.SecurityProtocolType]::Tls12
        $archive = Join-Path $temporary $archiveName
        $checksums = Join-Path $temporary 'checksums.txt'
        Write-Host "Downloading $archiveName..."
        Save-SessDownload "$($BaseUrl.TrimEnd('/'))/$archiveName" $archive
        Save-SessDownload "$($BaseUrl.TrimEnd('/'))/checksums.txt" $checksums
        $matchesForArchive = @(
            foreach ($line in [IO.File]::ReadAllLines($checksums)) {
                if ($line -match '^([a-fA-F0-9]{64})\s+\*?(.+)$' -and $Matches[2] -ceq $archiveName) {
                    $Matches[1]
                }
            }
        )
        if ($matchesForArchive.Count -ne 1) { throw "Missing or ambiguous checksum for $archiveName." }
        $actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash
        if ($actual -ne $matchesForArchive[0]) { throw "Checksum mismatch for $archiveName." }

        Add-Type -AssemblyName System.IO.Compression.FileSystem
        $zip = [IO.Compression.ZipFile]::OpenRead($archive)
        try {
            if ($zip.Entries.Count -ne 1 -or $zip.Entries[0].FullName -cne 'sess.exe') {
                throw 'Archive must contain only a regular sess.exe entry.'
            }
            $source = $zip.Entries[0].Open()
            try {
                $destination = [IO.File]::Create($staged)
                try { $source.CopyTo($destination) } finally { $destination.Dispose() }
            } finally { $source.Dispose() }
        } finally { $zip.Dispose() }
        if ((Get-Item -LiteralPath $staged -Force).Length -eq 0) { throw 'Archive contained an empty sess.exe.' }
        Assert-SessTarget $InstallDir $target
        if (Test-Path -LiteralPath $target) {
            [IO.File]::Replace($staged, $target, [NullString]::Value)
        } else {
            [IO.File]::Move($staged, $target)
        }
        $staged = $null
        Write-Host "Installed sess to $target"
        $active = Get-SessApplication
        if (-not $active -or -not [string]::Equals($active, $target, [StringComparison]::OrdinalIgnoreCase)) {
            if ($active) { Write-Host "PATH still selects another installation: $active" }
            Write-Host "Add $InstallDir to your user PATH, then open a new terminal."
            $quoted = $InstallDir.Replace("'", "''")
            Write-Host "For this PowerShell session: `$env:Path = '$quoted;' + `$env:Path"
        }
        Write-Host "Verify with 'Get-Command sess' and 'sess --version'. Session data is unchanged."
    } catch {
        throw "Installation failed: $($_.Exception.Message) Close running SESS processes; for protected or managed installs, use the original installation method or choose -InstallDir under your user profile."
    } finally {
        [Net.ServicePointManager]::SecurityProtocol = $oldTls
        if ($staged -and (Test-Path -LiteralPath $staged)) { Remove-Item -LiteralPath $staged -Force }
        if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Recurse -Force }
    }
}

Install-Sess
