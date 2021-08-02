#!/usr/bin/env bash
set -e

ARCH=$1

echo "****************************************"
echo "Creating tar file for Mac OS X $ARCH    "
echo "****************************************"

ROOTFS=${GO_SPACE}/bin/darwin_$ARCH/darwin
TAR_NAME=amazon-ssm-agent.tar.gz
DESTINATION=${GO_SPACE}/bin/amazon-ssm-agent-darwin-$ARCH-`cat ${GO_SPACE}/VERSION`.tar.gz
rm -rf ${ROOTFS}

echo "Creating darwin folders"

mkdir -p ${ROOTFS}/Library/LaunchDaemons

PROGRAM_FOLDER=${ROOTFS}/opt/aws/ssm

mkdir -p ${PROGRAM_FOLDER}
mkdir -p ${PROGRAM_FOLDER}/bin

echo "Copying application files"

cp ${GO_SPACE}/bin/darwin_$ARCH/amazon-ssm-agent ${PROGRAM_FOLDER}/bin/
cp ${GO_SPACE}/bin/darwin_$ARCH/ssm-agent-worker ${PROGRAM_FOLDER}/bin/
cp ${GO_SPACE}/bin/darwin_$ARCH/ssm-document-worker ${PROGRAM_FOLDER}/bin/
cp ${GO_SPACE}/bin/darwin_$ARCH/ssm-cli ${PROGRAM_FOLDER}/bin/
cp ${GO_SPACE}/bin/darwin_$ARCH/ssm-session-logger ${PROGRAM_FOLDER}/bin/
cp ${GO_SPACE}/bin/darwin_$ARCH/ssm-session-worker ${PROGRAM_FOLDER}/bin/

cp ${GO_SPACE}/seelog_unix.xml ${PROGRAM_FOLDER}/seelog.xml.template
cp ${GO_SPACE}/amazon-ssm-agent.json.template ${PROGRAM_FOLDER}/
cp ${GO_SPACE}/RELEASENOTES.md ${PROGRAM_FOLDER}/
cp ${GO_SPACE}/README.md ${PROGRAM_FOLDER}/
cp ${GO_SPACE}/packaging/darwin/com.amazon.aws.ssm.plist ${ROOTFS}/Library/LaunchDaemons/

echo "Setting permissions as required by launchd"

chmod 600 ${ROOTFS}/Library/LaunchDaemons/*

echo "Creating tar"
(
cd ${ROOTFS}
tar czf $TAR_NAME * --owner=0 --group=0
)

echo "Moving tar"
cp ${ROOTFS}/${TAR_NAME} ${DESTINATION}

echo "Archive created at ${ROOTFS}/${TAR_NAME} and a versioned copy is at ${DESTINATION}"
