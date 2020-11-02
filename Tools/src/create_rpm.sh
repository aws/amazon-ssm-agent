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

rm -rf ${BGO_SPACE}/bin/$FOLDER/linux

echo "Creating rpmbuild workspace $FOLDER"
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/rpmbuild/SPECS
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/rpmbuild/COORD_SOURCES
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/rpmbuild/DATA_SOURCES
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/rpmbuild/BUILD
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/rpmbuild/RPMS
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/rpmbuild/SRPMS
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/usr/bin/
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/etc/init/
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/etc/systemd/system/
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
mkdir -p ${BGO_SPACE}/bin/$FOLDER/linux/var/lib/amazon/ssm/


echo "Copying application files $FOLDER"

cp ${BGO_SPACE}/bin/$FOLDER/amazon-ssm-agent ${BGO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${BGO_SPACE}/bin/$FOLDER/ssm-agent-worker ${BGO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${BGO_SPACE}/bin/$FOLDER/ssm-document-worker ${BGO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${BGO_SPACE}/bin/$FOLDER/ssm-session-worker ${BGO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${BGO_SPACE}/bin/$FOLDER/ssm-session-logger ${BGO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${BGO_SPACE}/bin/$FOLDER/ssm-cli ${BGO_SPACE}/bin/$FOLDER/linux/usr/bin/
cp ${BGO_SPACE}/seelog_unix.xml ${BGO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/seelog.xml.template
cp ${BGO_SPACE}/amazon-ssm-agent.json.template ${BGO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/RELEASENOTES.md ${BGO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/README.md ${BGO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/NOTICE.md ${BGO_SPACE}/bin/$FOLDER/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/packaging/linux/amazon-ssm-agent.conf ${BGO_SPACE}/bin/$FOLDER/linux/etc/init/
cp ${BGO_SPACE}/packaging/linux/amazon-ssm-agent.service ${BGO_SPACE}/bin/$FOLDER/linux/etc/systemd/system/

echo "Creating the rpm package $FOLDER"

SPEC_FILE="${BGO_SPACE}/packaging/linux/amazon-ssm-agent.spec"
BUILD_ROOT="${BGO_SPACE}/bin/$FOLDER/linux"

rpmbuild -bb --target $TARGET --define "rpmversion `cat ${BGO_SPACE}/VERSION`" --define "_topdir bin/$FOLDER/linux/rpmbuild" --buildroot ${BUILD_ROOT} ${SPEC_FILE}
cp ${BGO_SPACE}/bin/$FOLDER/linux/rpmbuild/RPMS/$TARGET/*.rpm ${BGO_SPACE}/bin/$FOLDER/amazon-ssm-agent.rpm
rm -rf ${BGO_SPACE}/bin/$FOLDER/linux/rpmbuild/RPMS/$TARGET/*
