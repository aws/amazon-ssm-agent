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
cp ${BGO_SPACE}/bin/linux_386/ssm-document-worker ${BGO_SPACE}/bin/linux_386/linux/usr/bin/
cp ${BGO_SPACE}/bin/linux_386/ssm-session-worker ${BGO_SPACE}/bin/linux_386/linux/usr/bin/
cp ${BGO_SPACE}/bin/linux_386/ssm-session-logger ${BGO_SPACE}/bin/linux_386/linux/usr/bin/
cp ${BGO_SPACE}/bin/linux_386/ssm-cli ${BGO_SPACE}/bin/linux_386/linux/usr/bin/
cp ${BGO_SPACE}/seelog_unix.xml ${BGO_SPACE}/bin/linux_386/linux/etc/amazon/ssm/seelog.xml.template
cp ${BGO_SPACE}/amazon-ssm-agent.json.template ${BGO_SPACE}/bin/linux_386/linux/etc/amazon/ssm/
cp ${BGO_SPACE}/RELEASENOTES.md ${BGO_SPACE}/bin/linux_386/linux/etc/amazon/ssm/RELEASENOTES.md
cp ${BGO_SPACE}/README.md ${BGO_SPACE}/bin/linux_386/linux/etc/amazon/ssm/README.md
cp ${BGO_SPACE}/packaging/linux/amazon-ssm-agent.conf ${BGO_SPACE}/bin/linux_386/linux/etc/init/
cp ${BGO_SPACE}/packaging/linux/amazon-ssm-agent.service ${BGO_SPACE}/bin/linux_386/linux/etc/systemd/system/
cd ${BGO_SPACE}/bin/linux_386/linux/usr/bin/; strip --strip-unneeded amazon-ssm-agent; strip --strip-unneeded ssm-cli; strip --strip-unneeded ssm-document-worker; strip --strip-unneeded ssm-session-worker; strip --strip-unneeded ssm-session-logger; cd ~-

echo "Creating the rpm package"

SPEC_FILE="${BGO_SPACE}/packaging/linux/amazon-ssm-agent.spec"
BUILD_ROOT="${BGO_SPACE}/bin/linux_386/linux"

setarch i386 rpmbuild --target i386 -bb --define "rpmversion `cat ${BGO_SPACE}/VERSION`" --define "_topdir bin/linux_386/linux/rpmbuild" --buildroot ${BUILD_ROOT} ${SPEC_FILE}

echo "Copying rpm files to bin"

cp ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/RPMS/i386/*.rpm ${BGO_SPACE}/bin/
cp ${BGO_SPACE}/bin/linux_386/linux/rpmbuild/RPMS/i386/*.rpm ${BGO_SPACE}/bin/linux_386/amazon-ssm-agent.rpm
