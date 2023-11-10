 param(
    [string] $SetupCliPath
)

Write-Output "Starting data integrity check for ssm-setup-cli binary in windows"
Write-Output "Checking to see if ssm-setup-cli is signed"

$AmazonSubjectName = "*Amazon.com Services LLC*"

if ((Get-AuthenticodeSignature -FilePath $SetupCliPath).SignerCertificate.SubjectName.Name -like $AmazonSubjectName)
{
    Write-Output "Valid certificate found for SSM-Setup-CLI"
} else {
    Write-Output "Certificate invalid"
    Exit
}

if ((Get-AuthenticodeSignature -FilePath $SetupCliPath).Status -eq "Valid")
{
    Write-Output "Valid signature found for SSM-Setup-CLI"
} else {
    Write-Output "Signature invalid"
    Exit
}
Write-Output "Completed data integrity check for ssm-setup-cli binary in windows"