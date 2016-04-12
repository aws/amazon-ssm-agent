#!/usr/bin/env bash
echo "*************************************************"
echo "Creating rpm file for Amazon Linux and RHEL amd64"
echo "*************************************************"

rm -rf ${BGO_SPACE}/bin/linux_amd64/linux

echo "Creating rpmbuild workspace"

mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/rpmbuild/SPECS
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/rpmbuild/COORD_SOURCES
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/rpmbuild/DATA_SOURCES
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/rpmbuild/BUILD
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/rpmbuild/RPMS
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/rpmbuild/SRPMS
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/usr/bin/
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/etc/init/
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/etc/systemd/system/
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/etc/amazon/ssm/
mkdir -p ${BGO_SPACE}/bin/linux_amd64/linux/var/lib/amazon/ssm/

echo "Copying application files"

cp ${BGO_SPACE}/bin/linux_amd64/amazon-ssm-agent ${BGO_SPACE}/bin/linux_amd64/linux/usr/bin/
cp ${BGO_SPACE}/seelog.xml ${BGO_SPACE}/bin/linux_amd64/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/amazon-ssm-agent.json ${BGO_SPACE}/bin/linux_amd64/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/packaging/linux/amazon-ssm-agent.conf ${BGO_SPACE}/bin/linux_amd64/linux/etc/init/
cp ${BGO_SPACE}/packaging/linux/amazon-ssm-agent.service ${BGO_SPACE}/bin/linux_amd64/linux/etc/systemd/system/
cd ${BGO_SPACE}/bin/linux_amd64/linux/usr/bin/; strip --strip-unneeded amazon-ssm-agent ;cd ~-

echo "Creating the rpm package"

BUILD_ROOT="--buildroot ${BGO_SPACE}/bin/linux_amd64/linux"
SPEC_FILE="${BGO_SPACE}/packaging/linux/amazon-ssm-agent.spec"

rpmbuild -bb --define "rpmversion `cat ${BGO_SPACE}/VERSION`" --define "buildarch 'x86_64'" --define "_topdir bin/linux_amd64/linux/rpmbuild" ${BUILD_ROOT} ${SPEC_FILE}

echo "Copying rpm files to bin"

cp ${BGO_SPACE}/bin/linux_amd64/linux/rpmbuild/RPMS/x86_64/*.rpm ${BGO_SPACE}/bin/
cp ${BGO_SPACE}/bin/linux_amd64/linux/rpmbuild/RPMS/x86_64/*.rpm ${BGO_SPACE}/bin/linux_amd64/amazon-ssm-agent.rpm