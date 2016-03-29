#!/usr/bin/env bash
echo "*************************************************"
echo "Creating rpm file for Amazon Linux and RHEL 386"
echo "*************************************************"

rm -rf ${BGO_SPACE}/bin/linux_386/linux

echo "Creating rpmbuild workspace"

mkdir -p ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/SPECS
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/COORD_SOURCES
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/DATA_SOURCES
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/BUILD
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/RPMS
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/SRPMS
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/usr/bin/
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/etc/init/
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/etc/systemd/system/
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/etc/amazon/ssm/
mkdir -p ${BGO_SPACE}/bin/linux_386/linux/var/lib/amazon/ssm/

echo "Copying application files"

cp ${BGO_SPACE}/bin/linux_386/amazon-ssm-agent ${BGO_SPACE}/bin/linux_386/linux/usr/bin/
cp ${BGO_SPACE}/seelog.xml ${BGO_SPACE}/bin/linux_386/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/amazon-ssm-agent.json ${BGO_SPACE}/bin/linux_386/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/packaging/amazon-linux-ami/amazon-ssm-agent.conf ${BGO_SPACE}/bin/linux_386/linux/etc/init/
cp ${BGO_SPACE}/packaging/amazon-linux-ami/amazon-ssm-agent.service ${BGO_SPACE}/bin/linux_386/linux/etc/systemd/system/
cd ${BGO_SPACE}/bin/linux_386/linux/usr/bin/; strip --strip-unneeded amazon-ssm-agent ;cd ~-

echo "Creating the rpm package"

BUILD_ROOT="--buildroot ${BGO_SPACE}/bin/linux_386/linux"
SPEC_FILE="${BGO_SPACE}/packaging/amazon-linux-ami/amazon-ssm-agent.spec"

setarch i686 rpmbuild --target i686 -bb --define "rpmversion `cat ${BGO_SPACE}/VERSION`" --define "buildarch 'noarch'" --define "_topdir bin/linux_386/linux/rpmbuild" ${BUILD_ROOT} ${SPEC_FILE}
echo "Copying rpm files to bin"

cp ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/RPMS/noarch/*.rpm ${BGO_SPACE}/bin/
cp ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/RPMS/noarch/*.rpm ${BGO_SPACE}/bin/linux_386/amazon-ssm-agent.rpm