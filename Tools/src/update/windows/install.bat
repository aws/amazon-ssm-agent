@echo off
setlocal

set InstallingFolder="%PROGRAMFILES%\Amazon\SSM"
set AgentZipFile="%~dp0package.zip"

rem Installing Amazon SSM Agent from %AgentFile% to %InstallingFolder%
call :UnZip %AgentZipFile% %InstallingFolder%

rem Copy uninstall.bat to %InstallingFolder%
if exist "%~dp0uninstall.bat" xcopy "%~dp0uninstall.bat" %InstallingFolder%

goto :eof

:UnZip <File> <Destination>
    rem Create destination folder if not exist
    md %2
    rem Create VB script to unzip file
    set vbs="%TEMP%\_.vbs"
    if exist %vbs% del /f /q %vbs%
    >%vbs% echo Set fso = CreateObject("Scripting.FileSystemObject")
    >>%vbs% echo Set objShell = CreateObject("Shell.Application")
    >>%vbs% echo Set FilesInZip=objShell.NameSpace(%1).items
    >>%vbs% echo objShell.NameSpace(%2).CopyHere(FilesInZip)
    >>%vbs% echo Set fso = Nothing
    >>%vbs% echo Set objShell = Nothing
    rem Run VB script
    cscript //nologo %vbs%
    rem Delete VB script
    if exist %vbs% del /f /q %vbs%
