#!/bin/bash

FOLDER=$1

if [ -z "$FOLDER" ]
  then
    echo "Folder must be passed to this script"
    exit 1
fi

echo "************************************************************"
echo "Creating $FOLDER binary tar file"
echo "************************************************************"

tar --owner=0 --group=0 -zcvf ${GO_SPACE}/bin/${FOLDER}/amazon-ssm-agent-binaries.tar.gz  -C ${GO_SPACE}/bin/${FOLDER}/ amazon-ssm-agent ssm-agent-worker ssm-document-worker ssm-session-worker ssm-session-logger ssm-cli
