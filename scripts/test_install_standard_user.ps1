#requires -Version 5.1
[CmdletBinding()]
param([Parameter(Mandatory = $true)][string]$BinaryPath)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'This test requires Windows.' }
$principal = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'The test launcher needs administrator access to create a temporary standard account.'
}

$suffix = [Guid]::NewGuid().ToString('N').Substring(0, 10)
$user = "sess$suffix"
$taskName = "sess-installer-$suffix"
$directory = Join-Path $env:ProgramData $taskName
$password = 'Aa1!' + [Guid]::NewGuid().ToString('N') + [Guid]::NewGuid().ToString('N')
$created = $false
$registered = $false
try {
    New-LocalUser -Name $user -Password (ConvertTo-SecureString $password -AsPlainText -Force) -Description 'Temporary SESS installer test' | Out-Null
    $created = $true
    $usersGroup = Get-LocalGroup -SID 'S-1-5-32-545'
    Add-LocalGroupMember -Group $usersGroup -Member $user
    [IO.Directory]::CreateDirectory($directory) | Out-Null
    $acl = Get-Acl -LiteralPath $directory
    $rule = [Security.AccessControl.FileSystemAccessRule]::new(
        "$env:COMPUTERNAME\$user", 'Modify', 'ContainerInherit, ObjectInherit', 'None', 'Allow'
    )
    $acl.AddAccessRule($rule)
    Set-Acl -LiteralPath $directory -AclObject $acl
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'install.ps1') -Destination $directory
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'test_install.ps1') -Destination $directory
    Copy-Item -LiteralPath $BinaryPath -Destination (Join-Path $directory 'sess.exe')

    # No credentials are written to the test files or logs.
    $runner = @'
$ErrorActionPreference = 'Stop'
try {
    $principal = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
    if ($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'Installer test account unexpectedly has administrator privileges.'
    }
    & "$PSScriptRoot\test_install.ps1" -BinaryPath "$PSScriptRoot\sess.exe" *>&1 |
        Out-File -LiteralPath "$PSScriptRoot\test.log"
    [IO.File]::WriteAllText("$PSScriptRoot\result.txt", 'success')
} catch {
    $_ | Out-File -LiteralPath "$PSScriptRoot\test.log" -Append
    [IO.File]::WriteAllText("$PSScriptRoot\result.txt", 'failure')
    exit 1
}
'@
    $runnerPath = Join-Path $directory 'run.ps1'
    [IO.File]::WriteAllText($runnerPath, $runner)
    $shell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $action = New-ScheduledTaskAction -Execute $shell -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$runnerPath`""
    $settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Minutes 5) `
        -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
    & wevtutil.exe sl Microsoft-Windows-TaskScheduler/Operational /e:true
    if ($LASTEXITCODE -ne 0) { throw 'Could not enable Task Scheduler diagnostics.' }
    Register-ScheduledTask -TaskName $taskName -Action $action -Settings $settings `
        -User "$env:COMPUTERNAME\$user" -Password $password -RunLevel Limited | Out-Null
    $registered = $true
    Start-ScheduledTask -TaskName $taskName
    $result = Join-Path $directory 'result.txt'
    $deadline = [DateTime]::UtcNow.AddMinutes(5)
    while (-not (Test-Path -LiteralPath $result) -and [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Seconds 2
    }
    $log = Join-Path $directory 'test.log'
    if (Test-Path -LiteralPath $log) { Get-Content -LiteralPath $log }
    if (-not (Test-Path -LiteralPath $result)) {
        $info = Get-ScheduledTaskInfo -TaskName $taskName
        $task = Get-ScheduledTask -TaskName $taskName
        Get-WinEvent -LogName Microsoft-Windows-TaskScheduler/Operational -MaxEvents 50 |
            Where-Object { $_.Message -like "*$taskName*" } |
            Select-Object TimeCreated, Id, Message | Format-List
        throw "Standard-user test timed out; task state: $($task.State), task result: $($info.LastTaskResult)."
    }
    if ([IO.File]::ReadAllText($result) -ne 'success') { throw 'Standard-user installer tests failed.' }
    Write-Host 'Installer tests passed under a non-administrator Windows account.'
} finally {
    if ($registered) {
        Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }
    if ($created) { Remove-LocalUser -Name $user }
    $password = $null
    if (Test-Path -LiteralPath $directory) { Remove-Item -LiteralPath $directory -Recurse -Force }
}
