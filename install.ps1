#requires -Version 7.0
<#
.SYNOPSIS
Install or update Mafsil from an official release.
.DESCRIPTION
Selects the latest stable release unless -Version is supplied, verifies the
release installer against SHA256SUMS, then runs it. No workspace is selected.
#>
[CmdletBinding()]
param(
    [string]$Version = 'latest',
    [string]$Destination
)

& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'
    if (-not $IsWindows) { throw 'Use install.sh on Linux.' }
    if ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'X64') { throw 'Mafsil supports Windows x64.' }

    function Get-MafsilLatestLocation {
        $handler = [Net.Http.HttpClientHandler]::new()
        $handler.AllowAutoRedirect = $false
        $client = [Net.Http.HttpClient]::new($handler)
        $client.Timeout = [TimeSpan]::FromSeconds(30)
        try {
            $request = [Net.Http.HttpRequestMessage]::new([Net.Http.HttpMethod]::Head, 'https://github.com/AMD4x/Mafsil/releases/latest')
            try {
                $response = $client.SendAsync($request).GetAwaiter().GetResult()
                try {
                    if ([int]$response.StatusCode -notin @(301,302,303,307,308) -or $null -eq $response.Headers.Location) { throw 'Could not resolve the latest stable Mafsil release.' }
                    $target = [Uri]::new($request.RequestUri, $response.Headers.Location)
                    return $target.AbsoluteUri
                } finally { $response.Dispose() }
            } finally { $request.Dispose() }
        } finally { $client.Dispose() }
    }

    function Resolve-MafsilVersion([string]$Requested) {
        if ($Requested -eq 'latest') {
            $match = [Regex]::Match((Get-MafsilLatestLocation), '\Ahttps://github\.com/AMD4x/Mafsil/releases/tag/(v[0-9]+\.[0-9]+\.[0-9]+)\z')
            if (-not $match.Success) { throw 'The latest release returned an unexpected tag or destination.' }
            return $match.Groups[1].Value
        }
        if ($Requested -cnotmatch '\Av[0-9]+\.[0-9]+\.[0-9]+\z') { throw 'Use latest or an explicit vMAJOR.MINOR.PATCH version.' }
        return $Requested
    }

    function Receive-MafsilFile([string]$Uri, [string]$Path, [long]$Limit) {
        $handler = [Net.Http.HttpClientHandler]::new()
        $handler.AllowAutoRedirect = $false
        $client = [Net.Http.HttpClient]::new($handler)
        $client.Timeout = [TimeSpan]::FromSeconds(60)
        try {
            $current = [Uri]$Uri
            for ($redirects = 0; $redirects -le 5; $redirects++) {
                if ($current.Scheme -ne 'https') { throw 'Downloads must remain on HTTPS.' }
                $response = $client.GetAsync($current, [Net.Http.HttpCompletionOption]::ResponseHeadersRead).GetAwaiter().GetResult()
                try {
                    if ([int]$response.StatusCode -in @(301,302,303,307,308)) {
                        if ($null -eq $response.Headers.Location) { throw 'Download redirect has no destination.' }
                        $current = [Uri]::new($current, $response.Headers.Location)
                        continue
                    }
                    $response.EnsureSuccessStatusCode() | Out-Null
                    if ($response.Content.Headers.ContentLength -gt $Limit) { throw 'Download exceeds its size limit.' }
                    $inputStream = $response.Content.ReadAsStreamAsync().GetAwaiter().GetResult()
                    try {
                        $outputStream = [IO.File]::Open($Path, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
                        $timeout = [Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds(60))
                        try {
                            $buffer = [byte[]]::new(16384)
                            $total = 0
                            while (($count = $inputStream.ReadAsync($buffer, 0, $buffer.Length, $timeout.Token).GetAwaiter().GetResult()) -gt 0) {
                                $total += $count
                                if ($total -gt $Limit) { throw 'Download exceeds its size limit.' }
                                $outputStream.Write($buffer, 0, $count)
                            }
                        } finally { $outputStream.Dispose(); $timeout.Dispose() }
                    } finally { $inputStream.Dispose() }
                    return
                } finally { $response.Dispose() }
            }
            throw 'Too many download redirects.'
        } finally { $client.Dispose() }
    }

    function Get-MafsilInstallerDigest([string]$Manifest) {
        $seen = @{}
        foreach ($line in [IO.File]::ReadAllLines($Manifest)) {
            $match = [Regex]::Match($line, '\A([a-f0-9]{64})  ([A-Za-z0-9_.-]+)\z')
            if (-not $match.Success) { throw 'Malformed release checksum manifest.' }
            $name = $match.Groups[2].Value
            if ($seen.ContainsKey($name)) { throw 'Duplicate release checksum entry.' }
            $seen[$name] = $match.Groups[1].Value
        }
        if (-not $seen.ContainsKey('install.ps1')) { throw 'The release manifest has no Windows installer.' }
        return $seen['install.ps1']
    }

    $resolvedVersion = Resolve-MafsilVersion $Version
    Write-Output "Installing or updating Mafsil $resolvedVersion"
    $stage = Join-Path ([IO.Path]::GetTempPath()) ('mafsil-bootstrap-' + [Guid]::NewGuid().ToString('N'))
    if (Test-Path -LiteralPath $stage) { throw 'Temporary bootstrap directory already exists.' }
    [IO.Directory]::CreateDirectory($stage) | Out-Null
    try {
        $manifest = Join-Path $stage 'SHA256SUMS'
        $installer = Join-Path $stage 'install.ps1'
        $base = "https://github.com/AMD4x/Mafsil/releases/download/$resolvedVersion"
        Receive-MafsilFile "$base/SHA256SUMS" $manifest 65536
        Receive-MafsilFile "$base/install.ps1" $installer 1048576
        $expected = Get-MafsilInstallerDigest $manifest
        if ((Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected) { throw 'Release installer checksum mismatch.' }
        $arguments = @('-NoProfile', '-File', $installer, '-Version', $resolvedVersion)
        if ($Destination) { $arguments += @('-Destination', $Destination) }
        & (Join-Path $PSHOME 'pwsh.exe') @arguments
        if ($LASTEXITCODE -ne 0) { throw "Mafsil installer failed (exit $LASTEXITCODE)." }
    } finally {
        foreach ($name in @('SHA256SUMS', 'install.ps1')) {
            $path = Join-Path $stage $name
            if ([IO.File]::Exists($path)) {
                [IO.File]::SetAttributes($path, [IO.File]::GetAttributes($path) -band (-bnot [IO.FileAttributes]::ReadOnly))
                [IO.File]::Delete($path)
                if ([IO.File]::Exists($path)) { throw 'Bootstrap temporary file could not be removed.' }
            }
        }
        if ([IO.Directory]::GetFileSystemEntries($stage).Count -eq 0) { [IO.Directory]::Delete($stage, $false) }
    }
}
