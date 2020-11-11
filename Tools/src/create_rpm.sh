#!/usr/bin/env bash
set -e

FOLDER=$1

if [ -z "$FOLDER" ]
 then
   echo "Folder name must be provided"
   exit 1
fi

if [[ "$FOLDER" == *amd64 ]]; then
 TARGET=x86_64
elif [[ "$FOLDER" == *arm64 ]]; then
 TARGET=aarch64
elif [[ "$FOLDER" == *386 ]]; then
 TARGET=i386
else
 echo "Unsupported architecture"
 exit 1
fi

echo "************************************************************"
echo "Creating $FOLDER rpm file for Amazon Linux and RHEL $TARGET"
echo "************************************************************"

rm -rf ${GO_SPACE}/bin/$FOLDER/linux

echo "Creating rpmbuild workspace $FOLDER"
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/rpmbuild/SPECS
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/rpmbuild/COORD_SOURCES
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/rpmbuild/DATA_SOURCES
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/rpmbuild/BUILD
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/rpmbuild/RPMS
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/rpmbuild/SRPMS
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/usr/bin/
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/etc/init/
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/etc/systemd/system/
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
mkdir -p ${GO_SPACE}/bin/$FOLDER/linux/var/lib/amazon/ssm/


echo "Copying application files $FOLDER"

cp ${GO_SPACE}/bin/$FOLDER/amazon-ssm-agent ${GO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${GO_SPACE}/bin/$FOLDER/ssm-agent-worker ${GO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${GO_SPACE}/bin/$FOLDER/ssm-document-worker ${GO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${GO_SPACE}/bin/$FOLDER/ssm-session-worker ${GO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${GO_SPACE}/bin/$FOLDER/ssm-session-logger ${GO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${GO_SPACE}/bin/$FOLDER/ssm-cli ${GO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${GO_SPACE}/seelog_unix.xml ${GO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/seelog.xml.template
cp ${GO_SPACE}/amazon-ssm-agent.json.template ${GO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
cp ${GO_SPACE}/RELEASENOTES.md ${GO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
cp ${GO_SPACE}/README.md ${GO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
cp ${GO_SPACE}/NOTICE.md ${GO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
cp ${GO_SPACE}/packaging/linux/amazon-ssm-agent.conf ${GO_SPACE}/bin/$FOLDER/linux/etc/init/
cp ${GO_SPACE}/packaging/linux/amazon-ssm-agent.service ${GO_SPACE}/bin/$FOLDER/linux/etc/systemd/system/

echo "Creating the rpm package $FOLDER"

SPEC_FILE="${GO_SPACE}/packaging/linux/amazon-ssm-agent.spec"
BUILD_ROOT="${GO_SPACE}/bin/$FOLDER/linux"

rpmbuild -bb --target $TARGET --define "rpmversion `cat ${GO_SPACE}/VERSION`" --define "_topdir bin/$FOLDER/linux/rpmbuild" --buildroot ${BUILD_ROOT} ${SPEC_FILE}
cp ${GO_SPACE}/bin/$FOLDER/linux/rpmbuild/RPMS/$TARGET/*.rpm ${GO_SPACE}/bin/$FOLDER/amazon-ssm-agent.rpm
rm -rf ${GO_SPACE}/bin/$FOLDER/linux/rpmbuild/RPMS/$TARGET/*
