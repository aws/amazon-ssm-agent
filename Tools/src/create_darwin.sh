#!/usr/bin/env bash
echo "****************************************"
echo "Creating tar file for Mac OS X amd64    "
echo "****************************************"

ROOTFS=${BGO_SPACE}/bin/darwin_amd64/darwin
rm -rf ${ROOTFS}

echo "Creating darwin folders" 

mkdir -p ${ROOTFS}/Library/LaunchDaemons

PROGRAM_FOLDER=${ROOTFS}/opt/ssm

mkdir -p ${PROGRAM_FOLDER}
mkdir -p ${PROGRAM_FOLDER}/bin

echo "Copying application files"

cp ${BGO_SPACE}/bin/darwin_amd64/amazon-ssm-agent ${PROGRAM_FOLDER}/bin/
cp ${BGO_SPACE}/bin/darwin_amd64/ssm-cli ${PROGRAM_FOLDER}/bin/

cp ${BGO_SPACE}/seelog_unix.xml ${PROGRAM_FOLDER}/seelog.xml
cp ${BGO_SPACE}/amazon-ssm-agent.json.template ${PROGRAM_FOLDER}/
cp ${BGO_SPACE}/packaging/darwin/com.amazon.ec2.ssm.plist ${ROOTFS}/Library/LaunchDaemons/

echo "Setting permissions as required by launchd"

chmod 600 ${ROOTFS}/Library/LaunchDaemons/*

echo "Creating tar"
(
cd ${ROOTFS}
tar czf ssm-agent-darwin.tar.gz * --owner=0 --group=0
)
