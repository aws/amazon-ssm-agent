#!/usr/bin/env bash
echo "****************************************"
echo "Copying DomainJoin and CloudWatch binaries"
echo "****************************************"

BIN_FOLDER="$BGO_SPACE/bin/"
PLUGIN_BINARIES="`brazil-path run.configfarm.artifacts`/artifacts"

cp $PLUGIN_BINARIES/AWS.CloudWatch.exe $BIN_FOLDER
cp $PLUGIN_BINARIES/AWS.CloudWatch.exe.config $BIN_FOLDER
cp $PLUGIN_BINARIES/AWS.DomainJoin.exe $BIN_FOLDER
cp $PLUGIN_BINARIES/AWS.DomainJoin.exe.config $BIN_FOLDER
cp $PLUGIN_BINARIES/log4net.config $BIN_FOLDER