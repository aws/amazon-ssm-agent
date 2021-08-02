#!/usr/bin/env bash

ARCH=$1

cp ${GO_SPACE}/Tools/src/update/darwin/install_tar.sh ${GO_SPACE}/bin/darwin_$ARCH/darwin/install.sh
cp ${GO_SPACE}/Tools/src/update/darwin/uninstall.sh ${GO_SPACE}/bin/darwin_$ARCH/darwin/uninstall.sh

chmod 755 ${GO_SPACE}/bin/darwin_$ARCH/darwin/install.sh
chmod 755 ${GO_SPACE}/bin/darwin_$ARCH/darwin/uninstall.sh
chmod 755 ${GO_SPACE}/bin/darwin_$ARCH/updater

tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-darwin-$ARCH.tar.gz  -C ${GO_SPACE}/bin/darwin_$ARCH/darwin/ amazon-ssm-agent.tar.gz install.sh uninstall.sh

tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-darwin-$ARCH.tar.gz  -C ${GO_SPACE}/bin/darwin_$ARCH/ updater

rm ${GO_SPACE}/bin/darwin_$ARCH/darwin/install.sh
rm ${GO_SPACE}/bin/darwin_$ARCH/darwin/uninstall.sh
