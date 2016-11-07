param(
    # These parameters are for managed instances
    [switch] $Register,
    [string] $Code,
    [string] $Id,
    [string] $Region,

    # This switch is to disable SSMAgent after installation
    [switch] $Disabled
)

function Log-Info {
    param(
        [string] $message
    )
    Write-Host("[INFO] {0}" -f $message)
}

function Log-Warning {
    param(
        [string] $message
    )
    Write-Warning("{0}" -f $message)
}

$ServiceName = "AmazonSSMAgent"
$ServiceDesc = "Amazon SSM Agent"
$SourceName = "package"
$SourceZip = "$SourceName.zip"

$Destination = Join-Path $env:programFiles -ChildPath "Amazon" | Join-Path -ChildPath "SSM"
$UnInstaller = Join-Path $PSScriptRoot -ChildPath "uninstall.ps1"
$Executable = Join-Path $Destination -ChildPath "amazon-ssm-agent.exe"
$SourcePath = Join-Path $PSScriptRoot -ChildPath $SourceZip
$ExtractedPackage = Join-Path $Destination -ChildPath $SourceName

Log-Info("Checking if {0} exists" -f $SourceZip)
# Check if source package exists in current location
if(-not (Test-Path $SourcePath)) {
    Log-Warning("$SourceZip is not found.. exit!")
    Exit 1
}

# Execute UnInstaller
Invoke-Expression "& '$UnInstaller'"
if($LASTEXITCODE -gt 0) {
    Log-Warning("Uninstalling Amazon SSM Agent failed.. exit!")
    Exit 1
}

Log-Info("Installing Amazon SSM Agent begins")

# Extract source package to SSM location
Log-Info("Unpacking $ServiceName package from {0} to {1}" -f $SourceZip, $Destination)

$unpacked = $false;
if ($PSVersionTable.PSVersion.Major -ge 5) {
    try {
        # Nano doesn't support Expand-Archive yet, but plans to add it in future release.
        # Attempt to execute Expand-Archive to unzip the source package first.
        Expand-Archive $SourcePath -DestinationPath $Destination -Force

        # Set this TRUE to indicate the unpack is done
        $unpacked = $true;

        Log-Info("Successfully unpacked {0} and copied files to {1}" -f $SourceZip, $Destination)
    } catch {
        Log-Warning("Failed to unpack the package by Expand-Archive cmdlet..")
    }
}

# If unpack failed with Expand-Archive cmdlet, try it with [System.IO.Compression.ZipFile]::ExtractToDirectory
if (-not $unpacked) {
    Log-Info("Attempting again with [System.IO.Compression.ZipFile]::ExtractToDirectory")

    try {
        # Load [System.IO.Compression.FileSystem]
        Add-Type -AssemblyName System.IO.Compression.FileSystem
    } catch {
        # If failed, try to load [System.IO.Compression.ZipFile]
        Add-Type -AssemblyName System.IO.Compression.ZipFile
    }

    try {
        # Try to unpack the package by [System.IO.Compression.ZipFile]::ExtractToDirectory and move them to destination
        [System.IO.Compression.ZipFile]::ExtractToDirectory("$SourcePath", "$Destination")

        Log-Info("Successfully unpacked the package into {0}" -f $Destination)
    } catch {
        Log-Warning("Failed to unpack the package.. exit!")
        Exit 1
    }
}

# Check if unpacking created a directory, then move files and delete the directory.
if($unpacked -and (Test-Path $ExtractedPackage)) {
    Log-Info("Package is extracted to a directory. Moving files to destination and deleting the directory")
    $AllFilesUnderExtractedPackage = Join-Path $ExtractedPackage -ChildPath "*"
    Move-Item $AllFilesUnderExtractedPackage $Destination -Force
    Remove-Item $ExtractedPackage -Force
}

# Copy UnInstaller to the destination
Copy-Item $UnInstaller $Destination -Force

# Check if register is set in argument
if($Register) {
    # Start RegisterManagedInstance process
    Log-Info("RegisterManagedInstance begins")
    Invoke-Expression "& '$Executable' -register -code $Code -id $Id -region $Region"
}

# Register Amazon SSM Agent service to Windows service entry
Log-Info("Creating $ServiceName Service")
if(Get-Command sc.exe -ErrorAction SilentlyContinue) {
    try {
        $ErrorActionPreference = "Stop";
        if(-not $Disabled) {
            sc.exe create $ServiceName binpath= "$Executable" start= auto displayname= $ServiceDesc
        } else {
            sc.exe create $ServiceName binpath= "$Executable" start= disabled displayname= $ServiceDesc
            Log-Info("Amazon SSM Agent service is diabled")
        }
        sc.exe description $ServiceName $ServiceDesc
        sc.exe failureflag $ServiceName 1
        sc.exe failure $ServiceName reset= 86400 actions= restart/30000/restart/30000/restart/30000
    } catch {
        $ex = $Error[0].Exception
        Log-Warning("{0}.. exit!" -f $ex)
    }
} else {
    Log-Warning("Failed to create Amazon SSM Agent Service: sc.exe command not found.. exit!")
    Exit 1
}

if(-not $Disabled) {
    # Start service
    Log-Info("Starting Amazon SSM Agent service")
    try {
        $ErrorActionPreference = "Stop";
        net start $ServiceName
    } catch {
        $ex = $Error[0].Exception
        Log-Warning("{0}.. exit!" -f $ex)
        Exit 1
    }
} else {
    Log-Info("Amazon SSM Agent service didn't start")
}

Log-Info("Installing Amazon SSM Agent successfully ended!")
