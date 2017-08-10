#!/usr/bin/env bash
echo "****************************************"
echo "Copying DomainJoin and CloudWatch binaries"
echo "****************************************"

BIN_FOLDER="$BGO_SPACE/bin/"

brazil_build=$1

if [[ $brazil_build = true ]]; then
    PLUGIN_BINARIES="`brazil-path run.configfarm.artifacts`/artifacts"
else
    PLUGIN_BINARIES="$BGO_SPACE/packaging/dependencies"
fi

echo "Plugin binaries location is "$PLUGIN_BINARIES

cp $PLUGIN_BINARIES/AWS.DomainJoin.exe $BIN_FOLDER
cp $PLUGIN_BINARIES/AWS.DomainJoin.exe.config $BIN_FOLDER
cp $PLUGIN_BINARIES/log4net.config $BIN_FOLDER

cp -R $PLUGIN_BINARIES/awsCloudwatch/ $BIN_FOLDER