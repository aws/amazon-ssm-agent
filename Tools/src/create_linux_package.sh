#!/usr/bin/env bash

cp ${GO_SPACE}/Tools/src/update/linux/install.sh ${GO_SPACE}/bin/linux_amd64/
cp ${GO_SPACE}/Tools/src/update/linux/uninstall.sh ${GO_SPACE}/bin/linux_amd64/
cp ${GO_SPACE}/Tools/src/update/linux/install.sh ${GO_SPACE}/bin/linux_386/
cp ${GO_SPACE}/Tools/src/update/linux/uninstall.sh ${GO_SPACE}/bin/linux_386/
cp ${GO_SPACE}/Tools/src/update/linux/install.sh ${GO_SPACE}/bin/linux_arm64/
cp ${GO_SPACE}/Tools/src/update/linux/uninstall.sh ${GO_SPACE}/bin/linux_arm64/
cp ${GO_SPACE}/Tools/src/update/ubuntu/install.sh ${GO_SPACE}/bin/debian_amd64/
cp ${GO_SPACE}/Tools/src/update/ubuntu/uninstall.sh ${GO_SPACE}/bin/debian_amd64/
cp ${GO_SPACE}/Tools/src/update/ubuntu/snap-install.sh ${GO_SPACE}/bin/debian_amd64/
cp ${GO_SPACE}/Tools/src/update/ubuntu/snap-uninstall.sh ${GO_SPACE}/bin/debian_amd64/
cp ${GO_SPACE}/Tools/src/update/ubuntu/snap-install.sh ${GO_SPACE}/bin/debian_arm64/
cp ${GO_SPACE}/Tools/src/update/ubuntu/snap-uninstall.sh ${GO_SPACE}/bin/debian_arm64/
cp ${GO_SPACE}/Tools/src/update/ubuntu/install.sh ${GO_SPACE}/bin/debian_arm64/
cp ${GO_SPACE}/Tools/src/update/ubuntu/uninstall.sh ${GO_SPACE}/bin/debian_arm64/
cp ${GO_SPACE}/Tools/src/update/ubuntu/install.sh ${GO_SPACE}/bin/debian_386/
cp ${GO_SPACE}/Tools/src/update/ubuntu/uninstall.sh ${GO_SPACE}/bin/debian_386/
cp ${GO_SPACE}/Tools/src/update/ubuntu/install.sh ${GO_SPACE}/bin/debian_arm/
cp ${GO_SPACE}/Tools/src/update/ubuntu/uninstall.sh ${GO_SPACE}/bin/debian_arm/


chmod 755 ${GO_SPACE}/bin/linux_amd64/install.sh ${GO_SPACE}/bin/linux_amd64/uninstall.sh
chmod 755 ${GO_SPACE}/bin/linux_386/install.sh ${GO_SPACE}/bin/linux_386/uninstall.sh
chmod 755 ${GO_SPACE}/bin/linux_arm64/install.sh ${GO_SPACE}/bin/linux_arm64/uninstall.sh
chmod 755 ${GO_SPACE}/bin/debian_amd64/install.sh ${GO_SPACE}/bin/debian_amd64/uninstall.sh
chmod 755 ${GO_SPACE}/bin/debian_amd64/snap-install.sh ${GO_SPACE}/bin/debian_amd64/snap-uninstall.sh
chmod 755 ${GO_SPACE}/bin/debian_arm64/snap-install.sh ${GO_SPACE}/bin/debian_arm64/snap-uninstall.sh
chmod 755 ${GO_SPACE}/bin/debian_386/install.sh ${GO_SPACE}/bin/debian_386/uninstall.sh
chmod 755 ${GO_SPACE}/bin/debian_arm/install.sh ${GO_SPACE}/bin/debian_arm/uninstall.sh
chmod 755 ${GO_SPACE}/bin/debian_arm64/install.sh ${GO_SPACE}/bin/debian_arm64/uninstall.sh
chmod 755 ${GO_SPACE}/bin/linux_amd64/updater
chmod 755 ${GO_SPACE}/bin/linux_386/updater
chmod 755 ${GO_SPACE}/bin/linux_arm/updater
chmod 755 ${GO_SPACE}/bin/linux_arm64/updater

tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-linux-amd64.tar.gz  -C ${GO_SPACE}/bin/linux_amd64/ amazon-ssm-agent.rpm install.sh uninstall.sh
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-linux-386.tar.gz  -C ${GO_SPACE}/bin/linux_386/ amazon-ssm-agent.rpm install.sh uninstall.sh
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-linux-arm64.tar.gz  -C ${GO_SPACE}/bin/linux_arm64/ amazon-ssm-agent.rpm install.sh uninstall.sh
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-ubuntu-arm.tar.gz  -C ${GO_SPACE}/bin/debian_arm/ amazon-ssm-agent.deb install.sh uninstall.sh

# ubuntu is prepacked since snaps will be added later
tar -cvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-ubuntu-amd64.tar  -C ${GO_SPACE}/bin/debian_amd64/ amazon-ssm-agent.deb install.sh uninstall.sh snap-install.sh snap-uninstall.sh
tar -cvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-snap-amd64.tar  -C ${GO_SPACE}/bin/debian_amd64/ snap-install.sh snap-uninstall.sh
tar -cvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-ubuntu-arm64.tar  -C ${GO_SPACE}/bin/debian_arm64/ amazon-ssm-agent.deb install.sh uninstall.sh snap-install.sh snap-uninstall.sh
tar -cvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-snap-arm64.tar  -C ${GO_SPACE}/bin/debian_arm64/ snap-install.sh snap-uninstall.sh
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-ubuntu-386.tar.gz  -C ${GO_SPACE}/bin/debian_386/ amazon-ssm-agent.deb install.sh uninstall.sh


tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-linux-amd64.tar.gz  -C ${GO_SPACE}/bin/linux_amd64/ updater
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-ubuntu-amd64.tar.gz  -C ${GO_SPACE}/bin/linux_amd64/ updater
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-snap-amd64.tar.gz  -C ${GO_SPACE}/bin/linux_amd64/ updater
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-snap-arm64.tar.gz  -C ${GO_SPACE}/bin/linux_arm64/ updater
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-ubuntu-arm64.tar.gz  -C ${GO_SPACE}/bin/linux_arm64/ updater
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-linux-386.tar.gz  -C ${GO_SPACE}/bin/linux_386/ updater
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-linux-arm64.tar.gz  -C ${GO_SPACE}/bin/linux_arm64/ updater
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-ubuntu-386.tar.gz  -C ${GO_SPACE}/bin/linux_386/ updater
tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-ubuntu-arm.tar.gz  -C ${GO_SPACE}/bin/linux_arm/ updater

rm ${GO_SPACE}/bin/debian_amd64/install.sh
rm ${GO_SPACE}/bin/debian_amd64/uninstall.sh
rm ${GO_SPACE}/bin/debian_amd64/snap-install.sh
rm ${GO_SPACE}/bin/debian_amd64/snap-uninstall.sh
rm ${GO_SPACE}/bin/debian_arm64/snap-install.sh
rm ${GO_SPACE}/bin/debian_arm64/snap-uninstall.sh
rm ${GO_SPACE}/bin/debian_arm64/install.sh
rm ${GO_SPACE}/bin/debian_arm64/uninstall.sh
rm ${GO_SPACE}/bin/debian_386/install.sh
rm ${GO_SPACE}/bin/debian_386/uninstall.sh
rm ${GO_SPACE}/bin/debian_arm/install.sh
rm ${GO_SPACE}/bin/debian_arm/uninstall.sh
rm ${GO_SPACE}/bin/linux_amd64/install.sh
rm ${GO_SPACE}/bin/linux_amd64/uninstall.sh
rm ${GO_SPACE}/bin/linux_386/install.sh
rm ${GO_SPACE}/bin/linux_386/uninstall.sh
rm ${GO_SPACE}/bin/linux_arm64/install.sh
rm ${GO_SPACE}/bin/linux_arm64/uninstall.sh