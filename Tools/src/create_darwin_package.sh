#!/usr/bin/env bash

ARCH=$1

cp ${GO_SPACE}/Tools/src/update/darwin/install_tar.sh ${GO_SPACE}/bin/darwin_${ARCH}${DEBUG_FLAG}/darwin/install.sh
cp ${GO_SPACE}/Tools/src/update/darwin/uninstall.sh ${GO_SPACE}/bin/darwin_${ARCH}${DEBUG_FLAG}/darwin/uninstall.sh

chmod 755 ${GO_SPACE}/bin/darwin_${ARCH}${DEBUG_FLAG}/darwin/install.sh
chmod 755 ${GO_SPACE}/bin/darwin_${ARCH}${DEBUG_FLAG}/darwin/uninstall.sh
chmod 755 ${GO_SPACE}/bin/darwin_${ARCH}${DEBUG_FLAG}/updater

tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-darwin-${ARCH}${DEBUG_FLAG}.tar.gz  -C ${GO_SPACE}/bin/darwin_${ARCH}${DEBUG_FLAG}/darwin/ amazon-ssm-agent.tar.gz install.sh uninstall.sh

tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-darwin-$ARCH.tar.gz  -C ${GO_SPACE}/bin/darwin_${ARCH}${DEBUG_FLAG}/ updater

rm ${GO_SPACE}/bin/darwin_${ARCH}${DEBUG_FLAG}/darwin/install.sh
rm ${GO_SPACE}/bin/darwin_${ARCH}${DEBUG_FLAG}/darwin/uninstall.sh
