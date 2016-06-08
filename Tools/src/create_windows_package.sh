#!/usr/bin/env bash

BUILD_PATH_AMD64=${BGO_SPACE}/bin/windows_amd64
BUILD_PATH_386=${BGO_SPACE}/bin/windows_386
PACKAGE_PATH_AMD64=${BUILD_PATH_AMD64}/windows
PACKAGE_PATH_386=${BUILD_PATH_386}/windows

cp ${BGO_SPACE}/Tools/src/update/windows/install.bat ${PACKAGE_PATH_AMD64}/
cp ${BGO_SPACE}/Tools/src/update/windows/uninstall.bat ${PACKAGE_PATH_AMD64}/
cp ${BGO_SPACE}/Tools/src/update/windows/install.bat ${PACKAGE_PATH_386}/
cp ${BGO_SPACE}/Tools/src/update/windows/uninstall.bat ${PACKAGE_PATH_386}/

WINDOWS_AMD64_ZIP=${BGO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-windows-amd64.zip
zip -j ${WINDOWS_AMD64_ZIP} ${PACKAGE_PATH_AMD64}/package.zip
zip -j ${WINDOWS_AMD64_ZIP} ${PACKAGE_PATH_AMD64}/install.bat
zip -j ${WINDOWS_AMD64_ZIP} ${PACKAGE_PATH_AMD64}/uninstall.bat

WINDOWS_386_ZIP=${BGO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-windows-386.zip
zip -j ${WINDOWS_386_ZIP} ${PACKAGE_PATH_386}/package.zip
zip -j ${WINDOWS_386_ZIP} ${PACKAGE_PATH_386}/install.bat
zip -j ${WINDOWS_386_ZIP} ${PACKAGE_PATH_386}/uninstall.bat

WINDOWS_AMD64_UPDATER_ZIP=${BGO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-updater-windows-amd64.zip
zip -j ${WINDOWS_AMD64_UPDATER_ZIP} ${BUILD_PATH_AMD64}/updater.exe

WINDOWS_386_UPDATER_ZIP=${BGO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-updater-windows-386.zip
zip -j ${WINDOWS_386_UPDATER_ZIP} ${BUILD_PATH_386}/updater.exe
