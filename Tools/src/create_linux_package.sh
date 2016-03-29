#!/usr/bin/env bash

cp ${BGO_SPACE}/Tools/src/update/linux/install.sh ${BGO_SPACE}/bin/linux_amd64/
cp ${BGO_SPACE}/Tools/src/update/linux/uninstall.sh ${BGO_SPACE}/bin/linux_amd64/
cp ${BGO_SPACE}/Tools/src/update/linux/install.sh ${BGO_SPACE}/bin/linux_386/
cp ${BGO_SPACE}/Tools/src/update/linux/uninstall.sh ${BGO_SPACE}/bin/linux_386/
cp ${BGO_SPACE}/Tools/src/update/debian/install.sh ${BGO_SPACE}/bin/debian_amd64/
cp ${BGO_SPACE}/Tools/src/update/debian/uninstall.sh ${BGO_SPACE}/bin/debian_amd64/
cp ${BGO_SPACE}/Tools/src/update/debian/install.sh ${BGO_SPACE}/bin/debian_386/
cp ${BGO_SPACE}/Tools/src/update/debian/uninstall.sh ${BGO_SPACE}/bin/debian_386/

chmod 755 ${BGO_SPACE}/bin/linux_amd64/install.sh ${BGO_SPACE}/bin/linux_amd64/uninstall.sh
chmod 755 ${BGO_SPACE}/bin/linux_386/install.sh ${BGO_SPACE}/bin/linux_386/uninstall.sh
chmod 755 ${BGO_SPACE}/bin/debian_amd64/install.sh ${BGO_SPACE}/bin/debian_amd64/uninstall.sh
chmod 755 ${BGO_SPACE}/bin/debian_386/install.sh ${BGO_SPACE}/bin/debian_386/uninstall.sh
chmod 755 ${BGO_SPACE}/bin/linux_amd64/updater
chmod 755 ${BGO_SPACE}/bin/linux_386/updater

tar -zcvf ${BGO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-linux-amd64.tar.gz  -C ${BGO_SPACE}/bin/linux_amd64/ amazon-ssm-agent.rpm install.sh uninstall.sh
tar -zcvf ${BGO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-ubuntu-amd64.tar.gz  -C ${BGO_SPACE}/bin/debian_amd64/ amazon-ssm-agent.deb install.sh uninstall.sh
tar -zcvf ${BGO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-linux-386.tar.gz  -C ${BGO_SPACE}/bin/linux_386/ amazon-ssm-agent.rpm install.sh uninstall.sh
tar -zcvf ${BGO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-ubuntu-386.tar.gz  -C ${BGO_SPACE}/bin/debian_386/ amazon-ssm-agent.deb install.sh uninstall.sh

tar -zcvf ${BGO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-updater-linux-amd64.tar.gz  -C ${BGO_SPACE}/bin/linux_amd64/ updater
tar -zcvf ${BGO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-updater-ubuntu-amd64.tar.gz  -C ${BGO_SPACE}/bin/linux_amd64/ updater
tar -zcvf ${BGO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-updater-linux-386.tar.gz  -C ${BGO_SPACE}/bin/linux_386/ updater
tar -zcvf ${BGO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${BGO_SPACE}/VERSION`/amazon-ssm-agent-updater-ubuntu-386.tar.gz  -C ${BGO_SPACE}/bin/linux_386/ updater

rm ${BGO_SPACE}/bin/debian_amd64/install.sh
rm ${BGO_SPACE}/bin/debian_amd64/uninstall.sh
rm ${BGO_SPACE}/bin/debian_386/install.sh
rm ${BGO_SPACE}/bin/debian_386/uninstall.sh
rm ${BGO_SPACE}/bin/linux_amd64/install.sh
rm ${BGO_SPACE}/bin/linux_amd64/uninstall.sh
rm ${BGO_SPACE}/bin/linux_386/install.sh
rm ${BGO_SPACE}/bin/linux_386/uninstall.sh