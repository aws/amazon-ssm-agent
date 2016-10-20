#!/usr/bin/env bash

BUILD_PATH_AMD64=${BGO_SPACE}/bin/windows_amd64
PACKAGE_PATH_WINDOWS=${BUILD_PATH_AMD64}/windows
PACKAGE_PATH_NANO=${PACKAGE_PATH_WINDOWS}_nano

mkdir -p ${PACKAGE_PATH_NANO}

cp ${BGO_SPACE}/Tools/src/update/windows_nano/install.ps1 ${PACKAGE_PATH_NANO}/
cp ${BGO_SPACE}/Tools/src/update/windows_nano/uninstall.ps1 ${PACKAGE_PATH_NANO}/
cp ${PACKAGE_PATH_WINDOWS}/package.zip ${PACKAGE_PATH_NANO}/

WINDOWS_NANO_ZIP=${BGO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-windows-nano-amd64.zip
zip -j ${WINDOWS_NANO_ZIP} ${PACKAGE_PATH_NANO}/package.zip
zip -j ${WINDOWS_NANO_ZIP} ${PACKAGE_PATH_NANO}/install.ps1
zip -j ${WINDOWS_NANO_ZIP} ${PACKAGE_PATH_NANO}/uninstall.ps1

WINDOWS_NANO_UPDATE_ZIP=${BGO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-updater-windows-nano-amd64.zip
zip -j ${WINDOWS_NANO_UPDATE_ZIP} ${BUILD_PATH_AMD64}/updater.exe
