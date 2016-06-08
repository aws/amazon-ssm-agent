@echo off
setlocal

set DoRegister=
if /I not "%~1" equ "RegisterManagedInstance" goto UNINSTALL
set DoRegister=1
set RMICode=%~2
set RMIId=%~3
set RMIRegion=%~4

:UNINSTALL
rem Try to remove current installation.
if not exist "%~dp0uninstall.bat" echo [ERROR] uninsall.bat does not exists. & exit /b 1

call "%~dp0uninstall.bat"
if not %errorLevel% == 0 echo [ERROR] Failed when trying to remove current installation. & exit /b 1

:INSTALL
echo [INFO] Detecting administrative permissions...
net session >nul 2>&1
if not %errorLevel% == 0 echo [ERROR] Current permissions are inadequate. & goto exit /b 1
echo [INFO] Administrative permissions confirmed.

set ServiceName=AmazonSSMAgent
set InstallingFolder=%PROGRAMFILES%\Amazon\SSM
set AgentZipFile=%~dp0package.zip

echo [INFO] Copy Amazon SSM Agent from %AgentZipFile% to %InstallingFolder%.
call :UnZip "%AgentZipFile%" "%InstallingFolder%"

echo [INFO] Copy uninstall.bat to %InstallingFolder%.
if exist "%~dp0uninstall.bat" xcopy "%~dp0uninstall.bat" "%InstallingFolder%" /Y

echo [INFO] Register %ServiceName% as Windows service.
sc create %ServiceName% binpath= "%InstallingFolder%\amazon-ssm-agent.exe" start= auto displayname= "Amazon SSM Agent"
if not %errorlevel% == 0 echo [ERROR] Failed to register %ServiceName% as Windows service. & exit /b 1

echo [INFO] Add service description.
sc description %ServiceName% "Amazon SSM Agent"
if not %errorlevel% == 0 echo [WARN] Failed to add description for %ServiceName% service.

echo [INFO] Configure %ServiceName% recovery settings.
sc failure %ServiceName% reset= 86400 actions= restart/1000/restart/1000//1000
if not %errorlevel% == 0 echo [WARN] Failed to configure recovery settings for %ServiceName% service.

if not defined DoRegister goto START_SVC
if not exist "%InstallingFolder%\amazon-ssm-agent.exe" echo [ERROR] amazon-ssm-agent.exe not found. & exit /b 1
echo [INFO] Register managed instance.
"%InstallingFolder%\amazon-ssm-agent.exe" -register -code "%RMICode%" -id "%RMIId%" -region "%RMIRegion%"
if %errorlevel% == 0 goto START_SVC
echo [ERROR] Failed to register managed instance.
exit /b 1

:START_SVC
echo [INFO] Start service.
net start %ServiceName%
if %errorlevel% == 0 goto FINISH
echo [ERROR] Failed to start %ServiceName% service.
exit /b 1

:FINISH
exit /b 0

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
