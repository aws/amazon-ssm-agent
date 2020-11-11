#!/usr/bin/env bash

BUILD_PATH_AMD64=${GO_SPACE}/bin/windows_amd64
BUILD_PATH_386=${GO_SPACE}/bin/windows_386
PACKAGE_PATH_AMD64=${BUILD_PATH_AMD64}/windows
PACKAGE_PATH_386=${BUILD_PATH_386}/windows

cp ${GO_SPACE}/Tools/src/update/windows/install.bat ${PACKAGE_PATH_AMD64}/
cp ${GO_SPACE}/Tools/src/update/windows/uninstall.bat ${PACKAGE_PATH_AMD64}/
cp ${GO_SPACE}/Tools/src/update/windows/install.bat ${PACKAGE_PATH_386}/
cp ${GO_SPACE}/Tools/src/update/windows/uninstall.bat ${PACKAGE_PATH_386}/

WINDOWS_AMD64_ZIP=${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-windows-amd64.zip
zip -j ${WINDOWS_AMD64_ZIP} ${PACKAGE_PATH_AMD64}/package.zip
zip -j ${WINDOWS_AMD64_ZIP} ${PACKAGE_PATH_AMD64}/install.bat
zip -j ${WINDOWS_AMD64_ZIP} ${PACKAGE_PATH_AMD64}/uninstall.bat

WINDOWS_386_ZIP=${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-windows-386.zip
zip -j ${WINDOWS_386_ZIP} ${PACKAGE_PATH_386}/package.zip
zip -j ${WINDOWS_386_ZIP} ${PACKAGE_PATH_386}/install.bat
zip -j ${WINDOWS_386_ZIP} ${PACKAGE_PATH_386}/uninstall.bat

WINDOWS_AMD64_UPDATER_ZIP=${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-windows-amd64.zip
zip -j ${WINDOWS_AMD64_UPDATER_ZIP} ${BUILD_PATH_AMD64}/updater.exe

WINDOWS_386_UPDATER_ZIP=${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-windows-386.zip
zip -j ${WINDOWS_386_UPDATER_ZIP} ${BUILD_PATH_386}/updater.exe
