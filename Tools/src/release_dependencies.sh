#!/usr/bin/env bash
echo "********************************************"
echo "Release  Binary Dependencies for Open Source"
echo "********************************************"

PACKAGING_FOLDER="$BGO_SPACE/packaging/dependencies/"
PLUGIN_BINARIES="`brazil-path run.configfarm.artifacts`/artifacts"

cp $PLUGIN_BINARIES/AWS.DomainJoin.exe $PACKAGING_FOLDER
cp $PLUGIN_BINARIES/AWS.DomainJoin.exe.config $PACKAGING_FOLDER
cp $PLUGIN_BINARIES/log4net.config $PACKAGING_FOLDER
cp -R $PLUGIN_BINARIES/awsCloudwatch/ $PACKAGING_FOLDER