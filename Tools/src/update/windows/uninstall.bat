@echo off
setlocal

set ServiceName=AmazonSSMAgent

set DoRemoveProgramData=
if /I not "%~1" equ "RemoveProgramData" goto BEGIN
set DoRemoveProgramData=1

:BEGIN
echo [INFO] Detecting administrative permissions...
net session >nul 2>&1
if not %errorlevel% == 0 echo [ERROR] Current permissions are inadequate. & exit /b 1
echo [INFO] Administrative permissions confirmed.

echo [INFO] Looking for %ServiceName% service...
sc query %ServiceName% > nul
if %errorlevel% == 1060 echo [INFO] Service does not exists. & goto DEL_FILES
echo [INFO] Service found.

echo [INFO] Checking service status...
sc query %ServiceName:"=% | find "STOPPED"
if %errorlevel% == 0 echo [INFO] Service is stopped. & goto DEL_SERVICE
echo [INFO] Service is running.

echo [INFO] Stopping serivce...
net stop %ServiceName%
if not %errorlevel% == 0 echo [ERROR] Failed to stop service. & exit /b 1
echo [INFO] Service is stopped.

:DEL_SERVICE
echo [INFO] Delete from Windows service controller.
sc delete AmazonSSMAgent
if not %errorlevel% == 0 echo [ERROR] Failed to delete service. & exit /b 1

:DEL_FILES
set ProgramFilesAmazonFolder=%PROGRAMFILES%\Amazon
set ProgramFilesSSMFolder=%ProgramFilesAmazonFolder%\SSM
set CustomizedSeelog=%ProgramFilesSSMFolder%\seelog.xml
set CustomizedAppConfig=%ProgramFilesSSMFolder%\amazon-ssm-agent.json
set ProgramDataAmazonFolder=%PROGRAMDATA%\Amazon
set ProgramDataSSMFolder=%ProgramDataAmazonFolder%\SSM

:DEL_PROGRAMDATA
if not defined DoRemoveProgramData goto DEL_PROGRAMFILES

:DEL_SSM_PROGRAMDATA
if not exist "%ProgramDataSSMFolder%" goto DEL_AMAZON_PROGRAMDATA

echo [INFO] Delete %ProgramDataSSMFolder%.
rd /s/q "%ProgramDataSSMFolder%"

:DEL_AMAZON_PROGRAMDATA
if not exist "%ProgramDataAmazonFolder%" goto DEL_SSM_PROGRAMFILES

rem Delete ProgramData Amazon folder if it is empty.
for /f %%i in ('dir /b "%ProgramDataAmazonFolder%\*.*"') do (
    goto DEL_SSM_PROGRAMFILES
)
echo [INFO] Delete %ProgramDataAmazonFolder%.
rd /s/q "%ProgramDataAmazonFolder%"

:DEL_PROGRAMFILES
:DEL_SSM_PROGRAMFILES
if not exist "%ProgramFilesSSMFolder%" goto DEL_AMAZON_PROGRAMFILES
echo [INFO] Delete files under %ProgramFilesSSMFolder%.

rem Loop through folders and delete them.
for /f "delims=" %%i in ('dir /b /a:d "%ProgramFilesSSMFolder%\*.*"') do (
  rd /s/q "%ProgramFilesSSMFolder%\%%i"
)

rem Loop through non-folders, keep the customized files.
set HasCustomizedSettings=
for /f "delims=" %%i in ('dir /b /a:-d "%ProgramFilesSSMFolder%\*.*"') do (
  set IsCustomized=
  if /I "%ProgramFilesSSMFolder%\%%i" equ "%CustomizedSeelog%" set IsCustomized=1
  if /I "%ProgramFilesSSMFolder%\%%i" equ "%CustomizedAppConfig%" set IsCustomized=1
  if defined IsCustomized (
    set HasCustomizedSettings=1
    echo [INFO] Keep %ProgramFilesSSMFolder%\%%i.
  ) else (
    del "%ProgramFilesSSMFolder%\%%i"
  )
)

rem If customized files exists, do not delete the folder.
if defined HasCustomizedSettings goto FINISH

echo [INFO] Delete %ProgramFilesSSMFolder%.
rd /s/q "%ProgramFilesSSMFolder%"

:DEL_AMAZON_PROGRAMFILES
if not exist "%ProgramFilesAmazonFolder%" goto FINISH

rem If Amazon folder contains other content, exit
for /f %%i in ('dir /b "%ProgramFilesAmazonFolder%\*.*"') do (
    goto :FINISH
)

echo [INFO] Delete %ProgramFilesAmazonFolder%.
rd /s/q "%ProgramFilesAmazonFolder%"

:FINISH
exit /b 0
