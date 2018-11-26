#!/usr/bin/env bash
echo "*************************************************"
echo "Creating rpm file for Amazon Linux and RHEL arm64"
echo "*************************************************"

rm -rf ${BGO_SPACE}/bin/linux_arm64/linux

echo "Creating rpmbuild workspace"

mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/rpmbuild/SPECS
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/rpmbuild/COORD_SOURCES
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/rpmbuild/DATA_SOURCES
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/rpmbuild/BUILD
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/rpmbuild/RPMS
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/rpmbuild/SRPMS
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/usr/bin/
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/etc/init/
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/etc/systemd/system/
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/etc/amazon/ssm/
mkdir -p ${BGO_SPACE}/bin/linux_arm64/linux/var/lib/amazon/ssm/

echo "Copying application files"

cp ${BGO_SPACE}/bin/linux_arm64/amazon-ssm-agent ${BGO_SPACE}/bin/linux_arm64/linux/usr/bin/
cp ${BGO_SPACE}/bin/linux_arm64/ssm-document-worker ${BGO_SPACE}/bin/linux_arm64/linux/usr/bin/
cp ${BGO_SPACE}/bin/linux_arm64/ssm-session-worker ${BGO_SPACE}/bin/linux_arm64/linux/usr/bin/
cp ${BGO_SPACE}/bin/linux_arm64/ssm-session-logger ${BGO_SPACE}/bin/linux_arm64/linux/usr/bin/
cp ${BGO_SPACE}/bin/linux_arm64/ssm-cli ${BGO_SPACE}/bin/linux_arm64/linux/usr/bin/
cp ${BGO_SPACE}/seelog_unix.xml ${BGO_SPACE}/bin/linux_arm64/linux/etc/amazon/ssm/seelog.xml.template
cp ${BGO_SPACE}/amazon-ssm-agent.json.template ${BGO_SPACE}/bin/linux_arm64/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/RELEASENOTES.md ${BGO_SPACE}/bin/linux_arm64/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/README.md ${BGO_SPACE}/bin/linux_arm64/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/packaging/linux/amazon-ssm-agent.conf ${BGO_SPACE}/bin/linux_arm64/linux/etc/init/
cp ${BGO_SPACE}/packaging/linux/amazon-ssm-agent.service ${BGO_SPACE}/bin/linux_arm64/linux/etc/systemd/system/
cd ${BGO_SPACE}/bin/linux_arm64/linux/usr/bin/; strip --strip-unneeded amazon-ssm-agent; strip --strip-unneeded ssm-cli; strip --strip-unneeded ssm-document-worker; strip --strip-unneeded ssm-session-worker; strip --strip-unneeded ssm-session-logger; cd ~-

echo "Creating the rpm package"

SPEC_FILE="${BGO_SPACE}/packaging/linux/amazon-ssm-agent.spec"
BUILD_ROOT="${BGO_SPACE}/bin/linux_arm64/linux"

rpmbuild -bb --target aarch64 --define "rpmversion `cat ${BGO_SPACE}/VERSION`" --define "_topdir bin/linux_arm64/linux/rpmbuild" --buildroot ${BUILD_ROOT} ${SPEC_FILE}

echo "Copying rpm files to bin"

cp ${BGO_SPACE}/bin/linux_arm64/linux/rpmbuild/RPMS/aarch64/*.rpm ${BGO_SPACE}/bin/
cp ${BGO_SPACE}/bin/linux_arm64/linux/rpmbuild/RPMS/aarch64/*.rpm ${BGO_SPACE}/bin/linux_arm64/amazon-ssm-agent.rpm
