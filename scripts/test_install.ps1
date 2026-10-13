#requires -Version 5.1
[CmdletBinding()]
param([string]$BinaryPath)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$installer = Join-Path $PSScriptRoot 'install.ps1'
$root = Join-Path ([IO.Path]::GetTempPath()) ('sess-installer-test-' + [Guid]::NewGuid().ToString('N'))
$saved = @{}
foreach ($name in @('PATH', 'LOCALAPPDATA', 'USERPROFILE', 'SESS_INSTALL_DIR', 'SESS_VERSION',
                    'SESS_BASE_URL', 'SESS_REPO', 'PROCESSOR_ARCHITECTURE', 'PROCESSOR_ARCHITEW6432')) {
    $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}

function Assert-Test {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

function Invoke-TestInstaller {
    param([string]$Directory = '')
    try {
        $arguments = @{}
        if ($Directory) { $arguments.InstallDir = $Directory }
        $output = @(& $installer @arguments *>&1)
        return @{ Success = $true; Output = ($output | Out-String) }
    } catch {
        return @{ Success = $false; Output = $_.ToString() }
    }
}

function Write-TestArchive {
    param([string]$Entry = 'sess.exe')
    $payload = Join-Path $root 'payload'
    [IO.File]::WriteAllText($payload, 'new executable fixture')
    foreach ($architecture in @('amd64', 'arm64')) {
        $archive = Join-Path $assets "sess-windows-$architecture.zip"
        if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive }
        $zip = [IO.Compression.ZipFile]::Open($archive, [IO.Compression.ZipArchiveMode]::Create)
        try { [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $payload, $Entry) | Out-Null }
        finally { $zip.Dispose() }
    }
    $lines = foreach ($architecture in @('amd64', 'arm64')) {
        $name = "sess-windows-$architecture.zip"
        "$( (Get-FileHash -LiteralPath (Join-Path $assets $name) -Algorithm SHA256).Hash )  $name"
    }
    [IO.File]::WriteAllLines((Join-Path $assets 'checksums.txt'), [string[]]$lines)
}

try {
    if ($env:OS -ne 'Windows_NT') { throw 'Run Windows installer tests on Windows.' }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Directory]::CreateDirectory($root) | Out-Null
    $assets = Join-Path $root 'assets'
    [IO.Directory]::CreateDirectory($assets) | Out-Null
    $env:LOCALAPPDATA = Join-Path $root 'local app data'
    $env:USERPROFILE = Join-Path $root 'home'
    $env:PATH = Join-Path $root 'empty path'
    $env:SESS_INSTALL_DIR = ''
    $env:SESS_REPO = ''
    $env:SESS_VERSION = ''
    $env:SESS_BASE_URL = ([Uri]::new($assets + [IO.Path]::DirectorySeparatorChar, [UriKind]::Absolute)).AbsoluteUri.TrimEnd('/')
    $env:PROCESSOR_ARCHITECTURE = 'AMD64'
    $env:PROCESSOR_ARCHITEW6432 = ''
    $state = Join-Path $env:USERPROFILE '.sess-cli\sess.db'
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($state)) | Out-Null
    [IO.File]::WriteAllText($state, 'unchanged session data')
    Write-TestArchive

    $result = Invoke-TestInstaller
    Assert-Test $result.Success $result.Output
    $directory = Join-Path $env:LOCALAPPDATA 'Programs\SESS'
    $target = Join-Path $directory 'sess.exe'
    Assert-Test (Test-Path -LiteralPath $target) 'Default user install missing.'
    Assert-Test ($result.Output -match 'user PATH') 'Missing PATH instructions.'
    Assert-Test ($env:PATH -eq (Join-Path $root 'empty path')) 'Installer modified PATH.'
    Assert-Test ([IO.File]::ReadAllText($state) -eq 'unchanged session data') 'Session data changed.'
    Write-Host 'PASS default install and data preservation'

    [IO.File]::WriteAllText($target, 'old executable')
    $env:PATH = $directory
    $result = Invoke-TestInstaller
    Assert-Test $result.Success $result.Output
    Assert-Test ([IO.File]::ReadAllText($target) -eq 'new executable fixture') 'Existing install was not updated.'
    Write-Host 'PASS in-place update'

    $custom = Join-Path $root "custom directory [literal]'s"
    $result = Invoke-TestInstaller $custom
    Assert-Test $result.Success $result.Output
    Assert-Test ($result.Output -match 'another installation') 'Missing shadow warning.'
    Assert-Test (Test-Path -LiteralPath (Join-Path $custom 'sess.exe')) 'Custom install missing.'
    Write-Host 'PASS custom directory and shadow warning'

    [IO.File]::WriteAllText($target, 'old executable')
    $checksumPath = Join-Path $assets 'checksums.txt'
    $originalChecksums = [IO.File]::ReadAllText($checksumPath)
    [IO.File]::WriteAllText($checksumPath, (('0' * 64) + '  sess-windows-amd64.zip'))
    $result = Invoke-TestInstaller
    Assert-Test (-not $result.Success -and $result.Output -match 'Checksum mismatch') 'Bad checksum accepted.'
    Assert-Test ([IO.File]::ReadAllText($target) -eq 'old executable') 'Bad checksum changed old copy.'
    [IO.File]::WriteAllText($checksumPath, $originalChecksums)
    Write-Host 'PASS checksum failure preservation'

    $lock = [IO.File]::Open($target, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
    try {
        $result = Invoke-TestInstaller
        Assert-Test (-not $result.Success) 'Locked executable accepted.'
    } finally { $lock.Dispose() }
    Assert-Test ([IO.File]::ReadAllText($target) -eq 'old executable') 'Locked executable changed.'
    Write-Host 'PASS locked executable rejection'

    [IO.File]::SetAttributes($target, [IO.FileAttributes]::ReadOnly)
    try {
        $result = Invoke-TestInstaller
        Assert-Test (-not $result.Success -and $result.Output -match 'read-only') 'Read-only executable accepted.'
    } finally { [IO.File]::SetAttributes($target, [IO.FileAttributes]::Normal) }
    Write-Host 'PASS protected executable rejection'

    $junction = Join-Path $root 'managed junction'
    New-Item -ItemType Junction -Path $junction -Target $directory | Out-Null
    try {
        $result = Invoke-TestInstaller $junction
        Assert-Test (-not $result.Success -and $result.Output -match 'junction') 'Managed junction accepted.'
    } finally { [IO.Directory]::Delete($junction) }
    Write-Host 'PASS managed path rejection'

    Write-TestArchive '../sess.exe'
    $result = Invoke-TestInstaller
    Assert-Test (-not $result.Success -and $result.Output -match 'Archive must contain') 'Unsafe ZIP accepted.'
    Assert-Test ([IO.File]::ReadAllText($target) -eq 'old executable') 'Unsafe ZIP changed old copy.'
    Write-TestArchive
    Write-Host 'PASS archive layout rejection'

    $env:PROCESSOR_ARCHITEW6432 = 'ARM64'
    $result = Invoke-TestInstaller (Join-Path $root 'arm64')
    Assert-Test ($result.Success -and $result.Output -match 'windows-arm64.zip') 'Native ARM64 detection failed.'
    $env:PROCESSOR_ARCHITEW6432 = ''
    $env:PROCESSOR_ARCHITECTURE = 'x86'
    $result = Invoke-TestInstaller
    Assert-Test (-not $result.Success -and $result.Output -match 'Unsupported Windows architecture') 'Unsupported architecture accepted.'
    $env:PROCESSOR_ARCHITECTURE = 'AMD64'
    Write-Host 'PASS architecture selection'

    Assert-Test (@(Get-ChildItem -LiteralPath $root -Filter '.sess-install-*' -Recurse -Force).Count -eq 0) 'Staged files leaked.'
    if ($BinaryPath) {
        $binary = (Resolve-Path -LiteralPath $BinaryPath).Path
        $archive = Join-Path $assets 'sess-windows-amd64.zip'
        Remove-Item -LiteralPath $archive
        $zip = [IO.Compression.ZipFile]::Open($archive, [IO.Compression.ZipArchiveMode]::Create)
        try { [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $binary, 'sess.exe') | Out-Null }
        finally { $zip.Dispose() }
        [IO.File]::WriteAllText($checksumPath, ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash + '  sess-windows-amd64.zip'))
        $realDirectory = Join-Path $root 'real binary'
        foreach ($attempt in 1..2) {
            $result = Invoke-TestInstaller $realDirectory
            Assert-Test $result.Success $result.Output
            & (Join-Path $realDirectory 'sess.exe') --version
            Assert-Test ($LASTEXITCODE -eq 0) 'Installed binary does not run.'
        }
        Write-Host 'PASS real binary install and update'
    }
    Write-Host 'Windows installer tests passed.'
} finally {
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
    if (Test-Path -LiteralPath $root) { Remove-Item -LiteralPath $root -Recurse -Force }
}
