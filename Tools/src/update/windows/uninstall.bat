@echo off
setlocal

set AmazonFolder="%PROGRAMFILES:"=%\Amazon"
set SSMFolder="%AmazonFolder:"=%\SSM"

if not exist %SSMFolder% goto :CheckAmazonFolder

rem Delete SSM folder
rd /s/q %SSMFolder%

:CheckAmazonFolder
if not exist %AmazonFolder% goto :eof

rem If Amazon folder contains other content, exit
for /f %%i in ('dir /b "%AmazonFolder:"=%\*.*"') do (
    goto :eof
)

rem Amazon folder is empty, delete it
rd /q %AmazonFolder%
goto :eof
