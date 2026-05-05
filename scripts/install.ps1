# MegaCLI installer for Windows
# Usage: irm https://raw.githubusercontent.com/rorikonn/MegaCLI/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "rorikonn/MegaCLI"
$InstallDir = "$env:USERPROFILE\.megacli\bin"
$BinaryName = "megacli.exe"

function Get-LatestVersion {
    $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    return $release.tag_name -replace '^v', ''
}

function Get-Architecture {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64"   { return "x86_64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch" }
    }
}

function Install-MegaCLI {
    $version = Get-LatestVersion
    $arch = Get-Architecture

    $archiveName = "megacli_${version}_Windows_${arch}.zip"
    $downloadUrl = "https://github.com/$Repo/releases/download/v${version}/$archiveName"

    Write-Host "Installing MegaCLI v${version} (Windows/${arch})..."
    Write-Host "  -> $downloadUrl"

    # Create install directory
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

    # Download
    $tmpZip = "$env:TEMP\megacli.zip"
    $tmpDir = "$env:TEMP\megacli_extract"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $tmpZip -UseBasicParsing

    # Extract
    if (Test-Path $tmpDir) { Remove-Item -Recurse -Force $tmpDir }
    Expand-Archive -Path $tmpZip -DestinationPath $tmpDir -Force

    # Find and move binary
    $exe = Get-ChildItem -Path $tmpDir -Recurse -Filter $BinaryName | Select-Object -First 1
    if (-not $exe) { throw "Binary not found in archive" }
    Copy-Item -Path $exe.FullName -Destination "$InstallDir\$BinaryName" -Force

    # Cleanup
    Remove-Item -Force $tmpZip
    Remove-Item -Recurse -Force $tmpDir

    # Add to user PATH
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
        Write-Host "  -> Added $InstallDir to user PATH"
    }

    Write-Host ""
    Write-Host "[OK] MegaCLI installed to $InstallDir\$BinaryName" -ForegroundColor Green
    Write-Host "  Restart your terminal, then run: megacli --help"
}

Install-MegaCLI
