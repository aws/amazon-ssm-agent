#!/usr/bin/env bash

cp ${GO_SPACE}/Tools/src/update/darwin/install_tar.sh ${GO_SPACE}/bin/darwin_amd64/darwin/install.sh
cp ${GO_SPACE}/Tools/src/update/darwin/uninstall.sh ${GO_SPACE}/bin/darwin_amd64/darwin/uninstall.sh

chmod 755 ${GO_SPACE}/bin/darwin_amd64/darwin/install.sh
chmod 755 ${GO_SPACE}/bin/darwin_amd64/darwin/uninstall.sh
chmod 755 ${GO_SPACE}/bin/darwin_amd64/updater

tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-darwin-amd64.tar.gz  -C ${GO_SPACE}/bin/darwin_amd64/darwin/ amazon-ssm-agent.tar.gz install.sh uninstall.sh

tar -zcvf ${GO_SPACE}/bin/updates/amazon-ssm-agent-updater/`cat ${GO_SPACE}/VERSION`/amazon-ssm-agent-updater-darwin-amd64.tar.gz  -C ${GO_SPACE}/bin/darwin_amd64/ updater

rm ${GO_SPACE}/bin/darwin_amd64/darwin/install.sh
rm ${GO_SPACE}/bin/darwin_amd64/darwin/uninstall.sh
