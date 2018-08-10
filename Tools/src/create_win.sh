#!/usr/bin/env bash
echo "****************************************"
echo "Creating zip file for Windows amd64"
echo "****************************************"

BIN_FOLDER=${BGO_SPACE}/bin
BUILD_FOLDER=${BIN_FOLDER}/windows_amd64
PACKAGE_FOLDER=${BUILD_FOLDER}/windows
TOOLS_FOLDER=${BGO_SPACE}/Tools/src

rm -rf ${PACKAGE_FOLDER}

echo "Creating windows folders"

mkdir -p ${PACKAGE_FOLDER}

echo "Copying application files"

cp ${BUILD_FOLDER}/amazon-ssm-agent.exe ${PACKAGE_FOLDER}/amazon-ssm-agent.exe
cp ${BUILD_FOLDER}/ssm-document-worker.exe ${PACKAGE_FOLDER}/ssm-document-worker.exe
cp ${BUILD_FOLDER}/ssm-session-worker.exe ${PACKAGE_FOLDER}/ssm-session-worker.exe
cp ${BUILD_FOLDER}/ssm-session-logger.exe ${PACKAGE_FOLDER}/ssm-session-logger.exe
cp ${BUILD_FOLDER}/ssm-cli.exe ${PACKAGE_FOLDER}/ssm-cli.exe
cp ${BGO_SPACE}/seelog_windows.xml.template ${PACKAGE_FOLDER}/seelog.xml.template
cp ${BGO_SPACE}/amazon-ssm-agent.json.template ${PACKAGE_FOLDER}/amazon-ssm-agent.json.template

echo "Copying windows package config files"

cp ${TOOLS_FOLDER}/LICENSE ${PACKAGE_FOLDER}/LICENSE

echo "Constructing the zip package"

if [ -f ${PACKAGE_FOLDER}/amazon-ssm-agent.zip ]
then
    rm ${PACKAGE_FOLDER}/amazon-ssm-agent.zip
fi
cd ${PACKAGE_FOLDER}
zip -r package *
