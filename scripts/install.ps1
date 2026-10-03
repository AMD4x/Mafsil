#requires -Version 7.0
[CmdletBinding()]
param(
    [ValidateSet('Install', 'Uninstall', 'Status')][string]$Action = 'Install',
    [string]$Version = 'v0.1.0',
    [string]$Destination,
    [string]$BundleDirectory,
    [string]$ExpectedSHA256
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
if (-not $IsWindows) { throw 'Use install.sh on Linux.' }
if ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'X64') { throw 'This package targets Windows amd64.' }
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw 'Version must be an explicit vMAJOR.MINOR.PATCH release.' }
if ([string]::IsNullOrWhiteSpace($Destination)) {
    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) { throw 'Specify -Destination when LOCALAPPDATA is unavailable.' }
    $Destination = Join-Path $env:LOCALAPPDATA 'Programs/Mafsil'
}
$destinationFull = [IO.Path]::GetFullPath($Destination).TrimEnd('\', '/')
if ($destinationFull -eq [IO.Path]::GetPathRoot($destinationFull).TrimEnd('\', '/') -or $destinationFull.StartsWith('\\')) { throw 'Use a local directory below a drive root.' }
$parent = [IO.Path]::GetDirectoryName($destinationFull)
$markerName = 'mafsil.install.json'
$managed = @('mafsil.exe', 'LICENSE', 'THIRD_PARTY_NOTICES.md', 'install.ps1')

function Assert-NoReparse([string]$Path) {
    $itemPath = [IO.Path]::GetFullPath($Path)
    while ($itemPath) {
        if ([IO.File]::Exists($itemPath) -or [IO.Directory]::Exists($itemPath)) {
            if (([IO.File]::GetAttributes($itemPath) -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Refusing reparse path: $itemPath" }
            if ([IO.File]::Exists($itemPath) -and (Get-Item -LiteralPath $itemPath).LinkType -eq 'HardLink') { throw "Refusing hard-linked file: $itemPath" }
        }
        $next = [IO.Path]::GetDirectoryName($itemPath)
        if ($next -eq $itemPath) { break }
        $itemPath = $next
    }
}
function Get-Digest([string]$Path) { return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() }
function Receive-Asset([string]$Uri, [string]$Path) {
    $handler = [Net.Http.HttpClientHandler]::new()
    $handler.AllowAutoRedirect = $false
    $client = [Net.Http.HttpClient]::new($handler)
    $client.Timeout = [TimeSpan]::FromSeconds(120)
    try {
        $current = [Uri]$Uri
        for ($redirects = 0; $redirects -le 5; $redirects++) {
            if ($current.Scheme -ne 'https') { throw 'Asset requests must use HTTPS.' }
            $response = $client.GetAsync($current, [Net.Http.HttpCompletionOption]::ResponseHeadersRead).GetAwaiter().GetResult()
            try {
                if ([int]$response.StatusCode -in @(301,302,303,307,308)) {
                    $current = [Uri]::new($current, $response.Headers.Location)
                    continue
                }
                $response.EnsureSuccessStatusCode() | Out-Null
                if ($response.Content.Headers.ContentLength -gt 134217728) { throw 'Asset exceeds 128 MiB.' }
                $inputStream = $response.Content.ReadAsStreamAsync().GetAwaiter().GetResult()
                $outputStream = [IO.File]::Open($Path, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
                $timeout = [Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds(120))
                try {
                    $buffer = [byte[]]::new(65536)
                    $total = 0
                    while (($count = $inputStream.ReadAsync($buffer, 0, $buffer.Length, $timeout.Token).GetAwaiter().GetResult()) -gt 0) {
                        $total += $count
                        if ($total -gt 134217728) { throw 'Asset exceeds 128 MiB.' }
                        $outputStream.Write($buffer, 0, $count)
                    }
                } finally { $inputStream.Dispose(); $outputStream.Dispose(); $timeout.Dispose() }
                return
            } finally { $response.Dispose() }
        }
        throw 'Too many asset redirects.'
    } finally { $client.Dispose() }
}
function Remove-ExactFile([string]$Path) {
    if (-not [IO.File]::Exists($Path)) { return }
    Assert-NoReparse $Path
    $attributes = [IO.File]::GetAttributes($Path)
    [IO.File]::SetAttributes($Path, $attributes -band (-bnot [IO.FileAttributes]::ReadOnly))
    [IO.File]::Delete($Path)
    if ([IO.File]::Exists($Path)) { throw "File was not removed: $Path" }
}
function Read-Installation {
    $marker = Join-Path $destinationFull $markerName
    if (-not [IO.File]::Exists($marker)) { return $null }
    Assert-NoReparse $marker
    if ((Get-Item -LiteralPath $marker).Length -gt 16384) { throw 'Installation record exceeds its size limit.' }
    $record = [IO.File]::ReadAllText($marker) | ConvertFrom-Json -AsHashtable
    if ($record.product -ne 'Mafsil' -or $record.schema -ne 1 -or $record.files.Count -ne $managed.Count) { throw 'Unrecognized installation record.' }
    foreach ($name in $managed) {
        if (-not $record.files.ContainsKey($name) -or $record.files[$name] -notmatch '^[a-f0-9]{64}$') { throw 'Invalid installation file record.' }
        $path = Join-Path $destinationFull $name
        Assert-NoReparse $path
        if (-not [IO.File]::Exists($path) -or (Get-Digest $path) -ne $record.files[$name]) { throw "Managed file is missing or modified: $name. Preserve it and repair the installation explicitly." }
    }
    return $record
}

Assert-NoReparse $destinationFull
if ($Action -eq 'Status') {
    $record = Read-Installation
    [ordered]@{ installed = ($null -ne $record); destination = $destinationFull; version = $record.version } | ConvertTo-Json
    return
}
[IO.Directory]::CreateDirectory($parent) | Out-Null
Assert-NoReparse $parent
$lockPath = $destinationFull + '.install-lock'
try { $lock = [IO.File]::Open($lockPath, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None) }
catch { throw "Another installation may be running, or a previous run left $lockPath. Inspect before removing a stale lock." }
$stage = $null
$backup = $null
$committed = $false
$changed = [Collections.Generic.List[string]]::new()
$saved = [Collections.Generic.List[string]]::new()
try {
    $record = Read-Installation
    if ($Action -eq 'Uninstall' -and $null -eq $record) { Write-Output 'No managed Mafsil installation found.'; return }
    if ($null -eq $record) {
        foreach ($name in ($managed + $markerName)) {
            if (Test-Path -LiteralPath (Join-Path $destinationFull $name)) { throw "Refusing to overwrite unowned file: $name" }
        }
    }
    $stage = Join-Path $parent ('.mafsil-install-' + [Guid]::NewGuid().ToString('N'))
    $backup = Join-Path $stage 'previous'
    [IO.Directory]::CreateDirectory($backup) | Out-Null
    if ($Action -eq 'Install') {
        $asset = "mafsil_${Version}_windows_amd64.exe"
        $downloadNames = @($asset, 'SHA256SUMS', 'LICENSE', 'THIRD_PARTY_NOTICES.md', 'install.ps1')
        foreach ($name in $downloadNames) {
            $path = Join-Path $stage $name
            if ($BundleDirectory) {
                $source = Join-Path ([IO.Path]::GetFullPath($BundleDirectory)) $name
                Assert-NoReparse $source
                if ((Get-Item -LiteralPath $source).Length -gt 134217728) { throw 'Asset exceeds 128 MiB.' }
                [IO.File]::Copy($source, $path, $false)
            } else {
                $uri = "https://github.com/AMD4x/Mafsil/releases/download/$Version/$name"
                Receive-Asset $uri $path
            }
        }
        $sumFile = Join-Path $stage 'SHA256SUMS'
        if ((Get-Item -LiteralPath $sumFile).Length -gt 65536) { throw 'Checksum manifest is too large.' }
        $sums = @{}
        foreach ($line in [IO.File]::ReadAllLines($sumFile)) {
            if ($line -notmatch '^([a-fA-F0-9]{64})  ([A-Za-z0-9_.-]+)$') { throw 'Malformed checksum manifest.' }
            if ($sums.ContainsKey($Matches[2])) { throw 'Duplicate checksum entry.' }
            $sums[$Matches[2]] = $Matches[1].ToLowerInvariant()
        }
        foreach ($name in $downloadNames | Where-Object { $_ -ne 'SHA256SUMS' }) {
            if (-not $sums.ContainsKey($name) -or (Get-Digest (Join-Path $stage $name)) -ne $sums[$name]) { throw "Checksum mismatch: $name" }
        }
        if ($ExpectedSHA256 -and $ExpectedSHA256.ToLowerInvariant() -ne $sums[$asset]) { throw 'Binary digest differs from -ExpectedSHA256.' }
        [IO.File]::Move((Join-Path $stage $asset), (Join-Path $stage 'mafsil.exe'))
        $candidate = Join-Path $stage 'mafsil.exe'
        $start = [Diagnostics.ProcessStartInfo]::new($candidate)
        $start.ArgumentList.Add('version')
        $start.UseShellExecute = $false
        $start.CreateNoWindow = $true
        $start.RedirectStandardOutput = $true
        $start.RedirectStandardError = $true
        $probe = [Diagnostics.Process]::Start($start)
        try {
            $stdout = $probe.StandardOutput.ReadToEndAsync()
            $stderr = $probe.StandardError.ReadToEndAsync()
            if (-not $probe.WaitForExit(10000)) { $probe.Kill($true); $probe.WaitForExit(); throw 'Candidate version check timed out.' }
            $banner = $stdout.GetAwaiter().GetResult()
            if ($probe.ExitCode -ne 0 -or $banner -notmatch ('^Mafsil ' + [Regex]::Escape($Version.Substring(1)) + ' \(')) { throw 'Candidate failed its version check.' }
        } finally { $probe.Dispose() }
        $hashes = [ordered]@{}
        foreach ($name in $managed) { $hashes[$name] = Get-Digest (Join-Path $stage $name) }
        $newRecord = [ordered]@{ product = 'Mafsil'; schema = 1; version = $Version; files = $hashes }
        [IO.File]::WriteAllText((Join-Path $stage $markerName), ($newRecord | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
    }
    [IO.Directory]::CreateDirectory($destinationFull) | Out-Null
    Assert-NoReparse $destinationFull
    # Retain originals before replacing anything. Move never truncates a hard link.
    foreach ($name in ($managed + $markerName)) {
        $path = Join-Path $destinationFull $name
        if ([IO.File]::Exists($path)) { [IO.File]::Move($path, (Join-Path $backup $name)); $saved.Add($name) }
    }
    if ($Action -eq 'Install') {
        foreach ($name in ($managed + $markerName)) {
            [IO.File]::Move((Join-Path $stage $name), (Join-Path $destinationFull $name))
            $changed.Add($name)
        }
        $null = Read-Installation
    }
    $committed = $true
    if ($Action -eq 'Install') {
        Write-Output "Installed Mafsil $Version at $destinationFull"
        Write-Output "Run: & '$($destinationFull.Replace("'", "''"))\mafsil.exe' help"
        Write-Output 'No PATH, startup, service or registry settings were changed.'
    } else { Write-Output 'Removed managed Mafsil files. User configurations and unrelated files were preserved.' }
} catch {
    $cause = $_
    if (-not $committed -and $backup) {
        try {
            foreach ($name in $changed) { Remove-ExactFile (Join-Path $destinationFull $name) }
            foreach ($name in $saved) { [IO.File]::Move((Join-Path $backup $name), (Join-Path $destinationFull $name)) }
        } catch { throw "Rollback incomplete; retain $backup for manual recovery. Original error: $cause. Rollback: $_" }
    }
    throw $cause
} finally {
    # A failed rollback retains its backups. Cleanup is flat and file-specific.
    try {
        if ($stage -and [IO.Directory]::Exists($stage)) {
            if ($committed) { foreach ($name in ($managed + $markerName)) { Remove-ExactFile (Join-Path $backup $name) } }
            foreach ($name in ($managed + $markerName + @('SHA256SUMS', "mafsil_${Version}_windows_amd64.exe"))) { Remove-ExactFile (Join-Path $stage $name) }
            if ([IO.Directory]::Exists($backup) -and [IO.Directory]::GetFileSystemEntries($backup).Count -eq 0) { [IO.Directory]::Delete($backup, $false) }
            if ([IO.Directory]::GetFileSystemEntries($stage).Count -eq 0) { [IO.Directory]::Delete($stage, $false) }
        }
    } finally {
        $lock.Dispose()
        Remove-ExactFile $lockPath
    }
    if ($Action -eq 'Uninstall' -and [IO.Directory]::Exists($destinationFull) -and [IO.Directory]::GetFileSystemEntries($destinationFull).Count -eq 0) { [IO.Directory]::Delete($destinationFull, $false) }
}
