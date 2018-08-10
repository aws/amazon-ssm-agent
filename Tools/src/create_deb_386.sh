#!/usr/bin/env bash
echo "****************************************"
echo "Creating deb file for Ubuntu Linux 386"
echo "****************************************"

rm -rf ${BGO_SPACE}/bin/debian_386/debian

echo "Creating debian folders" 

mkdir -p ${BGO_SPACE}/bin/debian_386/debian/usr/bin/
mkdir -p ${BGO_SPACE}/bin/debian_386/debian/etc/init/
mkdir -p ${BGO_SPACE}/bin/debian_386/debian/etc/amazon/ssm/
mkdir -p ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/
mkdir -p ${BGO_SPACE}/bin/debian_386/debian/usr/share/lintian/overrides/
mkdir -p ${BGO_SPACE}/bin/debian_386/debian/var/lib/amazon/ssm/
mkdir -p ${BGO_SPACE}/bin/debian_386/debian/lib/systemd/system/

echo "Copying application files"

cp ${BGO_SPACE}/bin/linux_386/amazon-ssm-agent ${BGO_SPACE}/bin/debian_386/debian/usr/bin/
cp ${BGO_SPACE}/bin/linux_386/ssm-cli ${BGO_SPACE}/bin/debian_386/debian/usr/bin/
cp ${BGO_SPACE}/bin/linux_386/ssm-document-worker ${BGO_SPACE}/bin/debian_386/debian/usr/bin/
cp ${BGO_SPACE}/bin/linux_386/ssm-session-worker ${BGO_SPACE}/bin/debian_386/debian/usr/bin/
cp ${BGO_SPACE}/bin/linux_386/ssm-session-logger ${BGO_SPACE}/bin/debian_386/debian/usr/bin/
cd ${BGO_SPACE}/bin/debian_386/debian/usr/bin/; strip --strip-unneeded amazon-ssm-agent; strip --strip-unneeded ssm-cli; strip --strip-unneeded ssm-document-worker; strip --strip-unneeded ssm-session-worker; strip --strip-unneeded ssm-session-logger; cd ~-
cp ${BGO_SPACE}/seelog_unix.xml ${BGO_SPACE}/bin/debian_386/debian/etc/amazon/ssm/seelog.xml.template
cp ${BGO_SPACE}/amazon-ssm-agent.json.template ${BGO_SPACE}/bin/debian_386/debian/etc/amazon/ssm/
cp ${BGO_SPACE}/packaging/ubuntu/amazon-ssm-agent.conf ${BGO_SPACE}/bin/debian_386/debian/etc/init/
cp ${BGO_SPACE}/packaging/ubuntu/amazon-ssm-agent.service ${BGO_SPACE}/bin/debian_386/debian/lib/systemd/system/

echo "Copying debian package config files"

cp ${BGO_SPACE}/RELEASENOTES.md ${BGO_SPACE}/bin/debian_386/debian/etc/amazon/ssm/RELEASENOTES.md
cp ${BGO_SPACE}/README.md ${BGO_SPACE}/bin/debian_386/debian/etc/amazon/ssm/README.md
cp ${BGO_SPACE}/Tools/src/LICENSE ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/ssm/copyright
cp ${BGO_SPACE}/packaging/ubuntu/conffiles ${BGO_SPACE}/bin/debian_386/debian/
cp ${BGO_SPACE}/packaging/ubuntu/docs ${BGO_SPACE}/bin/debian_386/debian/
cp ${BGO_SPACE}/packaging/ubuntu/preinst ${BGO_SPACE}/bin/debian_386/debian/
cp ${BGO_SPACE}/packaging/ubuntu/postinst ${BGO_SPACE}/bin/debian_386/debian/
cp ${BGO_SPACE}/packaging/ubuntu/prerm ${BGO_SPACE}/bin/debian_386/debian/
cp ${BGO_SPACE}/packaging/ubuntu/lintian-overrides ${BGO_SPACE}/bin/debian_386/debian/usr/share/lintian/overrides/amazon-ssm-agent

echo "Constructing the control file"

echo 'Package: amazon-ssm-agent' > ${BGO_SPACE}/bin/debian_386/debian/control
echo 'Architecture: i386' >> ${BGO_SPACE}/bin/debian_386/debian/control
echo -n 'Version: ' >> ${BGO_SPACE}/bin/debian_386/debian/control
cat ${BGO_SPACE}/VERSION | tr -d "\n" >> ${BGO_SPACE}/bin/debian_386/debian/control
echo '-1' >> ${BGO_SPACE}/bin/debian_386/debian/control
cat ${BGO_SPACE}/packaging/ubuntu/control >> ${BGO_SPACE}/bin/debian_386/debian/control

echo "Constructing the changelog file"

echo -n 'amazon-ssm-agent (' > ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/changelog
cat VERSION | tr -d "\n"  >> ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/changelog
echo '-1) unstable; urgency=low' >> ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/changelog
cat ${BGO_SPACE}/packaging/ubuntu/changelog >> ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/changelog

cp ${BGO_SPACE}/packaging/ubuntu/changelog.Debian ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/
cp ${BGO_SPACE}/packaging/ubuntu/debian-binary ${BGO_SPACE}/bin/debian_386/debian/

echo "Setting permissions as required by debian"

cd ${BGO_SPACE}/bin/debian_386/; find ./debian -type d | xargs chmod 755; cd ~-

echo "Compressing changelog"

cd ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/; export GZIP=-9; tar cvzf changelog.gz changelog --owner=0 --group=0 ; cd ~-
cd ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/; export GZIP=-9; tar cvzf changelog.Debian.gz changelog.Debian --owner=0 --group=0; cd ~-

rm ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/changelog
rm ${BGO_SPACE}/bin/debian_386/debian/usr/share/doc/amazon-ssm-agent/changelog.Debian

echo "Creating tar"
# the below permissioning is required by debian
cd ${BGO_SPACE}/bin/debian_386/debian/; tar czf data.tar.gz usr etc lib --owner=0 --group=0 ; cd ~-
cd ${BGO_SPACE}/bin/debian_386/debian/; tar czf control.tar.gz control conffiles docs preinst postinst prerm --owner=0 --group=0 ; cd ~-

echo "Constructing the deb package"
ar r ${BGO_SPACE}/bin/debian_386/amazon-ssm-agent-`cat ${BGO_SPACE}/VERSION`-1.deb ${BGO_SPACE}/bin/debian_386/debian/debian-binary
ar r ${BGO_SPACE}/bin/debian_386/amazon-ssm-agent-`cat ${BGO_SPACE}/VERSION`-1.deb ${BGO_SPACE}/bin/debian_386/debian/control.tar.gz
ar r ${BGO_SPACE}/bin/debian_386/amazon-ssm-agent-`cat ${BGO_SPACE}/VERSION`-1.deb ${BGO_SPACE}/bin/debian_386/debian/data.tar.gz
cp ${BGO_SPACE}/bin/debian_386/amazon-ssm-agent-`cat ${BGO_SPACE}/VERSION`-1.deb ${BGO_SPACE}/bin/debian_386/amazon-ssm-agent.deb
mv ${BGO_SPACE}/bin/debian_386/amazon-ssm-agent-`cat ${BGO_SPACE}/VERSION`-1.deb ${BGO_SPACE}/bin/.