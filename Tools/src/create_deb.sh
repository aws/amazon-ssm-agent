#!/usr/bin/env bash
set -e

ARCH=$1

echo "****************************************"
echo "Creating deb file for Ubuntu Linux $ARCH"
echo "****************************************"

rm -rf ${BGO_SPACE}/bin/debian_${ARCH}/debian

if [[ "$ARCH" == "arm" ]]; then
  DEB_ARCH="armhf"
elif [[ "$ARCH" == "386" ]]; then
  DEB_ARCH="i386"
else
  DEB_ARCH=$ARCH
fi

echo "Creating debian folders debian_$ARCH"

mkdir -p ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/bin/
mkdir -p ${BGO_SPACE}/bin/debian_${ARCH}/debian/etc/init/
mkdir -p ${BGO_SPACE}/bin/debian_${ARCH}/debian/etc/amazon/ssm/
mkdir -p ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/
mkdir -p ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/lintian/overrides/
mkdir -p ${BGO_SPACE}/bin/debian_${ARCH}/debian/var/lib/amazon/ssm/
mkdir -p ${BGO_SPACE}/bin/debian_${ARCH}/debian/lib/systemd/system/

echo "Copying application files debian_$ARCH"

cp ${BGO_SPACE}/bin/linux_${ARCH}/amazon-ssm-agent ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/bin/
cp ${BGO_SPACE}/bin/linux_${ARCH}/ssm-agent-worker ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/bin/
cp ${BGO_SPACE}/bin/linux_${ARCH}/ssm-cli ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/bin/
cp ${BGO_SPACE}/bin/linux_${ARCH}/ssm-document-worker ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/bin/
cp ${BGO_SPACE}/bin/linux_${ARCH}/ssm-session-worker ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/bin/
cp ${BGO_SPACE}/bin/linux_${ARCH}/ssm-session-logger ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/bin/
cp ${BGO_SPACE}/seelog_unix.xml ${BGO_SPACE}/bin/debian_${ARCH}/debian/etc/amazon/ssm/seelog.xml.template
cp ${BGO_SPACE}/amazon-ssm-agent.json.template ${BGO_SPACE}/bin/debian_${ARCH}/debian/etc/amazon/ssm/
cp ${BGO_SPACE}/packaging/ubuntu/amazon-ssm-agent.conf ${BGO_SPACE}/bin/debian_${ARCH}/debian/etc/init/
cp ${BGO_SPACE}/packaging/ubuntu/amazon-ssm-agent.service ${BGO_SPACE}/bin/debian_${ARCH}/debian/lib/systemd/system/

echo "Copying debian package config files debian_$ARCH"

cp ${BGO_SPACE}/RELEASENOTES.md ${BGO_SPACE}/bin/debian_${ARCH}/debian/etc/amazon/ssm/RELEASENOTES.md
cp ${BGO_SPACE}/README.md ${BGO_SPACE}/bin/debian_${ARCH}/debian/etc/amazon/ssm/README.md
cp ${BGO_SPACE}/NOTICE.md ${BGO_SPACE}/bin/debian_${ARCH}/debian/etc/amazon/ssm/
cp ${BGO_SPACE}/Tools/src/LICENSE ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/copyright
cp ${BGO_SPACE}/packaging/ubuntu/conffiles ${BGO_SPACE}/bin/debian_${ARCH}/debian/
cp ${BGO_SPACE}/packaging/ubuntu/docs ${BGO_SPACE}/bin/debian_${ARCH}/debian/
cp ${BGO_SPACE}/packaging/ubuntu/preinst ${BGO_SPACE}/bin/debian_${ARCH}/debian/
cp ${BGO_SPACE}/packaging/ubuntu/postinst ${BGO_SPACE}/bin/debian_${ARCH}/debian/
cp ${BGO_SPACE}/packaging/ubuntu/prerm ${BGO_SPACE}/bin/debian_${ARCH}/debian/
cp ${BGO_SPACE}/packaging/ubuntu/lintian-overrides ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/lintian/overrides/amazon-ssm-agent

echo "Constructing the control file debian_$ARCH"

echo 'Package: amazon-ssm-agent' > ${BGO_SPACE}/bin/debian_${ARCH}/debian/control
echo "Architecture: ${DEB_ARCH}" >> ${BGO_SPACE}/bin/debian_${ARCH}/debian/control
echo -n 'Version: ' >> ${BGO_SPACE}/bin/debian_${ARCH}/debian/control
cat ${BGO_SPACE}/VERSION | tr -d "\n" >> ${BGO_SPACE}/bin/debian_${ARCH}/debian/control
echo '-1' >> ${BGO_SPACE}/bin/debian_${ARCH}/debian/control
cat ${BGO_SPACE}/packaging/ubuntu/control >> ${BGO_SPACE}/bin/debian_${ARCH}/debian/control

echo "Constructing the changelog file debian_$ARCH"

echo -n 'amazon-ssm-agent (' > ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/changelog
cat VERSION | tr -d "\n"  >> ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/changelog
echo '-1) unstable; urgency=low' >> ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/changelog
cat ${BGO_SPACE}/packaging/ubuntu/changelog >> ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/changelog

cp ${BGO_SPACE}/packaging/ubuntu/changelog.Debian ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/
cp ${BGO_SPACE}/packaging/ubuntu/debian-binary ${BGO_SPACE}/bin/debian_${ARCH}/debian/

echo "Setting permissions as required by debian_$ARCH"

cd ${BGO_SPACE}/bin/debian_${ARCH}/; find ./debian -type d | xargs chmod 755; cd ~-

echo "Compressing changelog for debian_$ARCH"

cd ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/; export GZIP=-9; tar cvzf changelog.gz changelog --owner=0 --group=0 ; cd ~-
cd ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/; export GZIP=-9; tar cvzf changelog.Debian.gz changelog.Debian --owner=0 --group=0; cd ~-

rm ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/changelog
rm ${BGO_SPACE}/bin/debian_${ARCH}/debian/usr/share/doc/amazon-ssm-agent/changelog.Debian

echo "Creating tar for debian_$ARCH"
# the below permissioning is required by debian
cd ${BGO_SPACE}/bin/debian_${ARCH}/debian/; tar czf data.tar.gz usr etc lib --owner=0 --group=0 ; cd ~-
cd ${BGO_SPACE}/bin/debian_${ARCH}/debian/; tar czf control.tar.gz control conffiles docs preinst postinst prerm --owner=0 --group=0 ; cd ~-

echo "Constructing the deb package debian_$ARCH"
ar r ${BGO_SPACE}/bin/debian_${ARCH}/amazon-ssm-agent.deb ${BGO_SPACE}/bin/debian_${ARCH}/debian/debian-binary
ar r ${BGO_SPACE}/bin/debian_${ARCH}/amazon-ssm-agent.deb ${BGO_SPACE}/bin/debian_${ARCH}/debian/control.tar.gz
ar r ${BGO_SPACE}/bin/debian_${ARCH}/amazon-ssm-agent.deb ${BGO_SPACE}/bin/debian_${ARCH}/debian/data.tar.gz

