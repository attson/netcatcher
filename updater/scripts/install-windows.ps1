# Args: -ParentPid <int> -StagedExe <path> -TargetExe <path>
# Replaces TargetExe with StagedExe after ParentPid exits, then relaunches.
param(
    [Parameter(Mandatory=$true)][int]$ParentPid,
    [Parameter(Mandatory=$true)][string]$StagedExe,
    [Parameter(Mandatory=$true)][string]$TargetExe
)

$ErrorActionPreference = 'Stop'

# Wait up to 30s for the parent to exit.
for ($i = 0; $i -lt 60; $i++) {
    if (-not (Get-Process -Id $ParentPid -ErrorAction SilentlyContinue)) { break }
    Start-Sleep -Milliseconds 500
}
if (Get-Process -Id $ParentPid -ErrorAction SilentlyContinue) {
    Write-Error "parent $ParentPid still running after 30s"
    exit 11
}

$old = "$TargetExe.old"
if (Test-Path $old) {
    Remove-Item $old -Force -ErrorAction SilentlyContinue
}

# Retry up to 3x for AV / file-lock contention.
$lastErr = $null
for ($i = 0; $i -lt 3; $i++) {
    try {
        Rename-Item -Path $TargetExe -NewName ([System.IO.Path]::GetFileName($old)) -ErrorAction Stop
        Copy-Item -Path $StagedExe -Destination $TargetExe -Force -ErrorAction Stop
        $lastErr = $null
        break
    } catch {
        $lastErr = $_
        Start-Sleep -Seconds 1
    }
}
if ($lastErr) {
    throw $lastErr
}

Start-Process -FilePath $TargetExe
